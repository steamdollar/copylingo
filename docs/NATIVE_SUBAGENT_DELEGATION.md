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
