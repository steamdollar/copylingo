# Gemini CLI External Executor Contract

## Purpose

Follow this contract whenever a main agent invokes you through Gemini CLI as an external executor.
The task-specific SSOT is the referenced `docs/todos/<task>.md`.

## Required Workflow

1. Read `AGENTS.md` and the referenced TODO document completely.
2. Inspect the current worktree before editing. It may contain valid partial edits from a previous execution.
3. Implement only the delegated scope. Do not duplicate completed work.
4. Stop and report the question if the specification requires a non-trivial decision.
5. Run every verification command listed in the TODO document.
6. Fix failures within scope and rerun verification.
7. Remove temporary files before reporting completion.

## Constraints

- Do not modify files outside the delegated scope.
- Do not revert existing changes unless the TODO document explicitly requires it.
- Do not claim completion without checking the actual files and verification results.
- Prefer structured artifacts and validators for high-volume content generation.
- Preserve Idempotency for database changes and external API calls.

## Completion Report

Report:

- Changed files
- Implemented scope
- Verification commands and results
- Remaining failures, questions, or partial work
