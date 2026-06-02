# Future Gemini CLI Invocation Stabilization

## Background And Goal

The current Gemini CLI external delegation workflow is documented in:

- `docs/GEMINI_CLI_DELEGATION.md`
- `docs/GEMINI_CLI_EXECUTION.md`

The main agent still invokes Gemini CLI manually.
During the 2026-06-02 N5 Vocabulary Material Catalog expansion, Gemini CLI returned temporary provider errors and response-level Tool Call errors:

- `429 MODEL_CAPACITY_EXHAUSTED`
- `503 UNAVAILABLE`
- Empty stream
- `malformed tool call`
- Missing required arguments such as `replace.old_string`
- A completion report without an actual file change

The goal is to add a small wrapper that makes invocation behavior consistent without blindly retrying potentially partial edits.

## Scope

Add:

- `scripts/run_gemini_executor.sh`
- `scripts/run_gemini_executor_test.sh`
- `docs/workthrough/YYMMDDhhmm_future_gemini_cli_invocation_stabilization.md`

Update only if usage documentation is needed:

- `docs/GEMINI_CLI_DELEGATION.md`

## Required Behavior

### CLI Interface

The wrapper receives a TODO document path:

```bash
scripts/run_gemini_executor.sh docs/todos/<task>.md
```

It invokes Gemini CLI with:

```bash
gemini --skip-trust --approval-mode yolo \
  -m gemini-3.1-flash-lite \
  -p "Read docs/GEMINI_CLI_EXECUTION.md and docs/todos/<task>.md. Execute the task exactly as specified."
```

Model order:

1. Default: `gemini-3.1-flash-lite`
2. Fallback: `gemini-2.5-flash`

Reject a missing or unreadable TODO path before invoking Gemini CLI.

### Temporary Provider Errors

Detect:

- `429`
- `MODEL_CAPACITY_EXHAUSTED`
- `503`
- `UNAVAILABLE`

Retry policy per model:

1. First failure: wait 5 seconds.
2. Second failure: wait 10 seconds.
3. Third failure: switch to the fallback model.
4. If the fallback exhausts the same sequence, exit non-zero and report the blocker.

Allow tests to override sleep duration through an environment variable such as `SUBAGENT_RETRY_SLEEP_SCALE=0`.

### Response Or Tool Call Errors

Detect at least:

- `Invalid stream`
- `malformed tool call`
- `required property`
- `could not find the string to replace`

For these errors:

1. Do not retry automatically.
2. Exit non-zero with a distinct message.
3. Print the recovery prompt from `docs/GEMINI_CLI_DELEGATION.md`.
4. Tell the main agent to inspect scoped `git status --short` and the diff before starting a recovery session.

Automatic recovery editing is explicitly out of scope. A previous invocation may have left valid partial edits, so a blind retry can duplicate or corrupt work.

### Output And Exit Status

- Preserve Gemini CLI stdout and stderr for diagnosis.
- Return zero only when Gemini CLI succeeds and no known error pattern is detected.
- Return non-zero for provider retry exhaustion, response-level Tool Call errors, invalid arguments, and Gemini CLI failures.
- Remove wrapper-owned temporary files on exit.

## Tests

Implement `scripts/run_gemini_executor_test.sh` with a temporary fake `gemini` executable placed first in `PATH`.

Cover:

- Missing TODO path fails before invocation.
- A successful Gemini call exits zero.
- Two provider failures followed by success retry the same model.
- Three provider failures switch to `gemini-2.5-flash`.
- Provider failures exhausted on both models exit non-zero.
- Each known Tool Call error exits non-zero without automatic retry.
- Normal non-zero Gemini CLI failure exits non-zero.

Use temporary directories and clean them with `trap`.
Do not call the real Gemini CLI from tests.

## Before And After

Before:

```bash
gemini --skip-trust --approval-mode yolo \
  -m gemini-3.1-flash-lite \
  -p "Read docs/GEMINI_CLI_EXECUTION.md and docs/todos/<task>.md. Execute the task exactly as specified."
```

After:

```bash
scripts/run_gemini_executor.sh docs/todos/<task>.md
```

## Verification

```bash
bash -n scripts/run_gemini_executor.sh
bash -n scripts/run_gemini_executor_test.sh
scripts/run_gemini_executor_test.sh
git diff --check
make test
```

## Do Not Change

- Do not automate recovery edits after a Tool Call error.
- Do not parse or modify delegated source files in the wrapper.
- Do not add a third-party dependency.
- Do not change `AGENTS.md`, `GEMINI.md`, or application code unless the user explicitly expands the scope.
- Do not invoke the real Gemini CLI in tests.

## Decisions Already Made

- Use a Bash wrapper under `scripts/`.
- Automate provider retries only when retrying is safe.
- Detect Tool Call errors but require the main agent to inspect partial edits before recovery.
- Keep retry policy aligned with `docs/GEMINI_CLI_DELEGATION.md`.
