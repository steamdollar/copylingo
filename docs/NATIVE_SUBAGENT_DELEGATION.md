# Native Subagent Delegation Protocol

## Purpose

Use native runtime subagents for bounded tasks that can run independently or in parallel.
Native spawn is the default delegation mechanism when the runtime exposes `multi_agent_v1`.

Gemini CLI is a separate external executor. Use `docs/GEMINI_CLI_DELEGATION.md` only when Gemini-specific execution is intentional, such as consuming Gemini quota for bulk content generation.

## Native Tools

- `spawn_agent`: create a child agent
- `send_input`: send follow-up instructions or redirect work
- `wait_agent`: wait only when the critical path needs the result
- `close_agent`: close agents that are no longer needed

## Delegate When

Delegate a task when:

- The user explicitly requests delegation, subagents, or parallel agent work.
- The subtask is concrete, bounded, and self-contained.
- The main agent can continue meaningful non-overlapping work while the child agent runs.
- A code-editing worker can own a disjoint set of files.

Keep work local when:

- The next main-agent action is blocked on the same task.
- The task is tightly coupled to local edits.
- The subtask requires a non-trivial decision not yet approved by the user.

## Delegation ROI Gate

Delegation (native subagent or Gemini) is a token-saving tool, not a default. Judge cost vs. benefit before delegating:

- First size the work with a cheap local scan (`rg`, `git diff`, `go test`) to estimate candidate count and task nature.
- If candidates are ≤10 or the task is a mechanical regex/pattern search, the main agent handles it directly.
- Delegate only when candidates are many or classification cost is high — semantic review, code-flow analysis, test-gap analysis.
- Scope the delegated input: prefer "classify this candidate list" over "scan the whole codebase".
- A subagent result is not the final judgment. The main agent re-verifies key candidates and owns the final decision and verification.

### Executor Selection

Once delegation is justified, choose the executor:

- **Default: native subagent.** For bulk mechanical work (large reads, classification, summarization), override the model down to **Haiku** — a clear cost reason per "Runtime Model".
- **External Gemini CLI (=agy)** only when one of:
  1. The user enabled conserve mode or explicitly requested it. Usage-headroom judgment belongs to the user — the agent cannot query Max plan usage.
  2. Estimated batch token volume exceeds the threshold (gauge: **~50k tokens**, calibratable). Estimate `≈ Σ(items × per-item input+output)`; generation is output-dominated, bulk read is input-dominated.
- When switching to the external executor, **report the estimate that triggered it** (e.g. "80 items × ~700 ≈ 56k > 50k"). Never judge "usage is tight" yourself — the in-session `tokens left` signal is the context window, not plan usage. React only to explicit in-band rate-limit warnings.

## Roles

- `explorer`: answer a specific codebase question without editing files.
- `worker`: implement a bounded patch with explicit file ownership.
- `default`: use only when neither specialized role fits.

## Spawn Prompt Checklist

For workers, include:

- Concrete task and acceptance criteria
- Owned files or modules
- Required verification commands
- A reminder that other agents may edit the codebase concurrently
- A reminder not to revert changes made by others
- A request to list changed files in the final report

For explorers, ask one focused question and request file references.

## Runtime Model

Native subagents inherit the parent model by default.
Do not override the model unless the user requests it or the task has a clear cost or capability reason.
Bulk mechanical work (large reads, classification, summarization) is a clear cost reason — prefer the **Haiku** tier for it, and reserve the parent (Opus/Sonnet) tier for judgment-bearing subtasks.

## Integration

After a child agent completes:

1. Review its report and uploaded changes.
2. Check the scoped diff.
3. Integrate or refine the result.
4. Run the project verification required by `AGENTS.md`.
5. Close the child agent when it is no longer needed.

## Relationship To Gemini CLI

Do not use terminal-based Gemini CLI execution as a substitute for native spawn.
Use Gemini CLI external delegation only when it is an intentional executor choice.
