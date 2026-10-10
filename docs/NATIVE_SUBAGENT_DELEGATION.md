# Native Subagent Delegation Protocol

## Purpose

Use the runtime's native subagents for bounded tasks that can run independently or in parallel.
Delegate only when the ROI gate below passes; otherwise the main agent does the work itself.

## Runtime Mapping

Claude Code and Codex are both main agents (`AGENTS.md` §2). The rules below are runtime-neutral; this table maps them to each runtime's tools.

| concept | Claude Code | Codex (`multi_agent_v1`) |
|---|---|---|
| spawn | `Agent` tool with `subagent_type` | `spawn_agent` |
| follow-up / redirect | `SendMessage` | `send_input` |
| wait | completion notification (background by default) | `wait_agent` |
| close | ends on completion | `close_agent` |
| read-only role | `Explore` or any read-only agent type the runtime lists | `explorer` |
| editing role | `general-purpose` or any editing agent type the runtime lists | `worker` |
| cheaper tier for bulk work | Haiku (`model` override) | a cheaper model, if the runtime offers one |

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

Delegation is a token-saving tool, not a default. Judge cost vs. benefit before delegating:

- First size the work with a cheap local scan (`rg`, `git diff`, `go test`) to estimate candidate count and task nature.
- If candidates are ≤10 or the task is a mechanical regex/pattern search, the main agent handles it directly.
- Delegate only when candidates are many or classification cost is high — semantic review, code-flow analysis, test-gap analysis.
- Scope the delegated input: prefer "classify this candidate list" over "scan the whole codebase".
- A subagent result is not the final judgment. The main agent re-verifies key candidates and owns the final decision and verification.

## Model Tier

Subagents inherit the parent model by default.
Override it only when the user requests it or the task has a clear cost or capability reason.
Bulk mechanical work (large reads, classification, summarization) is a clear cost reason: use the cheaper tier from the mapping table, and keep the parent tier for judgment-bearing subtasks.

## Spawn Prompt Checklist

For editing workers, include:

- Concrete task and acceptance criteria
- Owned files or modules
- Required verification commands
- A reminder that other agents may edit the codebase concurrently
- A reminder not to revert changes made by others
- A request to list changed files in the final report

For read-only agents, ask one focused question and request file references.

## Integration

After a child agent completes:

1. Review its report and changes.
2. Check the scoped diff.
3. Integrate or refine the result.
4. Run the project verification required by `AGENTS.md`.
5. Close the child agent when the runtime requires it.
