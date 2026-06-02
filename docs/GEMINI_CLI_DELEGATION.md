# Gemini CLI External Delegation Protocol

## Purpose

Use this protocol when a main agent delegates high-volume, low-complexity work to Gemini CLI as an external OS process.
This is not the native subagent protocol. Prefer native spawn when the runtime exposes it and Gemini-specific execution is unnecessary.

## Use External Delegation When

Prefer Gemini CLI external delegation when all conditions are true:

- The task is repetitive but does not require architectural decisions.
- Scope and acceptance criteria fit in `docs/todos/<task>.md`.
- Tests, validators, or count checks can verify the result mechanically.
- A partial failure can be detected and recovered safely.

Keep the task with the main agent or use native spawn when any condition is true:

- It requires schema design, architecture selection, or another non-trivial decision.
- Requirements are ambiguous or require user confirmation.
- Partial application cannot be verified safely.
- The error cost is high, such as security, authentication, or payment logic.

## Model Selection

Use Gemini CLI models in this order:

1. Default: `gemini-3.1-flash-lite`
2. Fallback: `gemini-2.5-flash`

On `404 ModelNotFoundError`, do not retry the same model. Switch to the fallback immediately.

## Prepare The Task

1. Write a self-contained `docs/todos/<task>.md`.
2. Include scope, forbidden changes, acceptance criteria, and verification commands.
3. For bulk content, prefer generating an independent artifact such as JSONL and validating it before updating source files.
4. For database changes or external API calls, specify Idempotency requirements.

## Dispatch

Keep the prompt short. Use the TODO document as the task SSOT and the Gemini CLI execution contract as the executor SSOT.

```bash
gemini --skip-trust --approval-mode yolo \
  -m gemini-3.1-flash-lite \
  -p "Read docs/GEMINI_CLI_EXECUTION.md and docs/todos/<task>.md. Execute the task exactly as specified."
```

Fallback:

```bash
gemini --skip-trust --approval-mode yolo \
  -m gemini-2.5-flash \
  -p "Read docs/GEMINI_CLI_EXECUTION.md and docs/todos/<task>.md. Execute the task exactly as specified."
```

The main agent must not trust the report alone. Inspect the scoped diff and rerun the project-level checks required by `AGENTS.md`.

## Error Handling

### Temporary Provider Errors: `429`, `503`

Treat `429 MODEL_CAPACITY_EXHAUSTED` and `503 UNAVAILABLE` as temporary provider errors.

1. Wait 5 seconds and retry the same model.
2. On the second failure, wait 10 seconds and retry the same model.
3. After the third failure, switch to the fallback model.
4. If the fallback exhausts the same retry sequence, stop and report the blocker to the user.

### Response Or Tool Call Errors

Examples:

- Empty stream
- `malformed tool call`
- Missing required tool arguments such as `replace.old_string`
- Success report without an actual file change
- Command timeout or abnormal process exit

Do not treat these as throttling. Do not immediately repeat the same prompt because a partial edit may already exist.

1. Inspect scoped `git status --short` and the diff.
2. Check for generated files, temporary files, and partial edits.
3. Preserve valid existing changes. Do not revert them automatically.
4. Start a fresh Gemini CLI session with the recovery prompt.

Recovery prompt:

```text
Read docs/GEMINI_CLI_EXECUTION.md and docs/todos/<task>.md.
The worktree may contain partial edits from a previous execution.
Inspect existing changes first. Do not duplicate completed work.
Complete only the remaining work and run the required verification commands.
```

If the recovery session repeats the same type of Tool Call error:

1. Stop direct editing of long source files.
2. Split the task into smaller units or generate an independent artifact such as JSONL.
3. Update or replace the TODO document with the revised workflow.
4. Retry once with a fresh session.
5. If the second recovery fails, report the blocker and ask the user whether the main agent should take over.

### Stop Without Retry

Do not retry:

- Authentication errors: `401`, `403`
- A non-trivial decision not covered by the task specification
- Changes outside the delegated scope
- Data corruption whose recovery cannot be verified mechanically

## Main Agent Checklist

After the subagent exits:

- Confirm the TODO acceptance criteria.
- Check for out-of-scope changes.
- Check for leftover temporary files.
- Confirm that the subagent ran its tests or validators.
- Inspect the scoped diff.
- Rerun `make test` for code, migration, or configuration changes.

## Operational Note

During the 2026-06-02 N5 Vocabulary Material Catalog expansion, repeated direct edits of a long source file caused provider capacity errors, Tool Call errors, and expensive review loops.
For bulk content, prefer: independent artifact generation, validator checks, then final source update.
