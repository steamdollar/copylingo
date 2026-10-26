# CopyLingo — Agent Contract & Doc Router

> The shared **contract** for every AI agent working on CopyLingo — the SSOT for roles, work protocol, and design criteria — and a **router** to the detail docs linked below. Agents load it directly (e.g. Codex) or through a thin per-agent overlay (e.g. `CLAUDE.md`). Don't copy detail into this file; link to the owning doc.

---

## 1. Precedence over personal config

When an agent's personal global config (e.g. `~/.claude/*`, `~/.codex/*`) conflicts with this contract, **this contract wins** (work protocol, coding conventions, delegation topology, etc.). Non-conflicting personal workflow (output language, tone, review procedure, per-turn reporting format, etc.) still applies.

---

## 2. Agent role matrix

| actor | responsibility |
|---|---|
| **User** | **Final decision-maker.** Agents propose, execute, and review, but **must get user confirmation on any non-trivial decision** (design, implementation direction, ADR adoption, TODO split, etc.). |
| **Main agent** (whichever agent the user starts the session with, e.g. Claude Code or Codex) | Carries one task end to end — **design · implement · verify · review**. Agent-specific usage patterns go to that agent's overlay file. |
| **Native subagent** (spawned by the main agent) | Runs bounded, self-contained subtasks; the main agent re-verifies results. When and how to delegate: [`docs/NATIVE_SUBAGENT_DELEGATION.md`](docs/NATIVE_SUBAGENT_DELEGATION.md). |

---

## 3. Work protocol — Case classification

A user request falls into one of these, and the deliverable and procedure differ by class. **Most requests start at Case 0 and escalate explicitly to A/B/C as needed — never change cases implicitly.**

> **Pre-branch — micro-change: the user handles it directly**
>
> For a **very small change** (e.g. one character, one variable name, a single typo) that's inefficient in agent tokens/time, don't do it — say **"It's more token-efficient to fix this yourself."** Only proceed if the user insists.

### Case 0. Investigate / review / answer (default)

> Answering a question, or analyzing/reviewing/explaining code, queries, design, or docs **without changing** code or docs.

- **Procedure**: confirm facts with as much search/read as needed **before answering** (no guessing). No separate deliverable (doc/code).

### Case A. Decision (ADR)

> Discussing a **decision about how the codebase is built** — system architecture, implementation approach, tradeoffs, etc.

1. Discuss with the user thoroughly. State tradeoffs at the assumed scale (see §4).
2. **Once the decision settles, the agent immediately adds an entry to the latest range file under `docs/adr/` without waiting to be asked** (background / decision / consequences). The user often forgets to update ADRs, so do it proactively.
   - **Separate-file rule**: if the entry chunk is large or the decision is significant to the project, put it in a **separate file** (keeping the series numbering, e.g. `ADR-031_032_listening_audio_pipeline.md`) and leave a **pointer stub** (number + one-line summary + link) in the range file.
3. If code changes follow, continue into Case B.

### Case B. Writing code

> Once scope is set, handling **plan → implement → verify → close** in one flow.

1. **Start**: check "🔨 진행 중" items in `STATUS.md` — judge relevance to the current request.
2. **Plan**: for non-trivial work, agree the plan with the user before implementing.
3. **Implement**: follow the `internal/` layer structure and coding conventions (§5).
4. **Verify**: `make test` is **required** for code/migration/config changes. Docs-only work may skip it; say so in the final report.
   - **Go formatting before tests/handoff**: apply `goparams` to the Go files you created or edited, per [the agent formatting procedure](docs/CONVENTIONS.md#agent-go-formatting).
   - **Runtime restart**: if the change must take effect in the local runtime (Go server, Mini App static assets, config), restart the relevant instance with the matching target in the [`Makefile`](Makefile) header manifest (e.g. `make restart-app`, which also health-checks). Restart DB/Redis/Tunnel only when you changed that component directly.
5. **Close**:
   - **`STATUS.md`**: move "🔨 진행 중" → "📝 최근 완료" **only when this request completes that in-progress item itself**. An unrelated side task (e.g. doc cleanup, an incidental finding) leaves STATUS.md alone or adds a single line under "📝 최근 완료". Keep only the **last 30 days** there; drop older rows when adding one (git log and `docs/workthrough/` keep the history).
   - **Workthrough**: create `docs/workthrough/YYMM/YYMMDDhhmm_<job>.md` **only when it records something found nowhere else** (not in an ADR, the code, or `git log`): ① a non-obvious root cause, incident, or lesson; ② measurement/verification evidence for a significant feature; ③ an operational or migration procedure someone will need again; ④ a significant design choice too small for an ADR. A changed-file list plus "make test passed" is not a reason — the commit message covers it.

**Language**: implementation plans and workthroughs are written in **Korean**.

### Case C. Splitting off and delegating a TODO

> During review/implementation, an anomaly/improvement **outside current scope** is found, and handling it now would blow up the scope — so it is only recorded as a TODO.

- **Owners**: split decision — user · plan doc — the current session's main agent · execution — a main agent in a separate session.

#### Splitting (current session)

1. **Always write `docs/todos/<task>.md` as a self-contained plan doc** (no one-line-summary escape hatch): background/purpose, files to change with before/after snippets, verification method (`make test`, etc.), off-limits areas, and decisions already made.
2. If an ambiguous decision point comes up while planning, **settle it with the user now and pin it in the doc** (so the executing agent doesn't have to re-ask).
3. Register a one-line summary + doc link in `STATUS.md`:
   `- [ ] <one-line summary> — see [docs/todos/<file>.md](docs/todos/<file>.md)`

#### Execution (separate session)

1. Find the plan path in `STATUS.md` → read `docs/todos/<task>.md` carefully. If clear, start immediately and **follow Case B steps 3 (implement) → 4 (verify) → 5 (close) as-is**; if any judgment beyond what the plan pins is needed, handle it as **C → B** below.
2. **Extra handling at Case C close** (on top of Case B close):
   - remove that TODO's checkbox item from `STATUS.md` (separate from Case B's "진행 중 → 최근 완료" rule)
   - delete `docs/todos/<task>.md` (preserved in git history)

### Transitions between cases

| transition | trigger | handling |
|---|---|---|
| **B → A** | a new **non-trivial decision** affecting assumed scale/architecture appears mid-implementation | **pause implementation** → settle it via Case A → return to Case B |
| **B → C** | an anomaly/improvement outside scope is found | don't stop the current implementation; split a TODO via Case C → resume Case B |
| **C → B** | execution needs a non-trivial decision the plan didn't specify, or the scope turns out far larger than the plan | stop immediately and report to the user → the user either augments the plan and resumes, or promotes it to a Case B in the main session |

---

## 4. Project character & design criteria ⚠️

> **Every architecture/refactor decision in this project is judged against this section.**

- CopyLingo is a dual-purpose project — **(a) real foreign-language learning + (b) portfolio** — and **when coding, (b) takes priority**.
- There is only 1 real user, but architecture/refactor decisions are judged **assuming this system serves tens of thousands of users**.
- Why: gaining experience handling larger-scale systems is one of this project's core purposes.

### How to apply

- **Right-size under the scale assumption and state the judgment with tradeoffs.** "It's fine, single user" or "YAGNI" is not a default without basis — and neither is pinning in a pattern without load justification (no cargo cult; this is not "distributed systems everywhere"). Patterns that genuinely matter at scale (cache SSOT, event stream/outbox, CQRS, async workers, etc.) are chosen deliberately.
- **Adding complexity and declining to are both first-class judgments.** The portfolio signal is right-sizing with explicit tradeoffs, not complexity itself; a non-trivial decision *not* to add something is also ADR-worthy.

### Decision principles

1. **Single-user optimization applies only to feature scope** — no social features, no monetization nudges (hearts, etc.).
2. **Push-based learning** — the bot sends sessions first; the user doesn't come to it.
3. **AI usage** — the LLM answers learners' questions and grades answers; TTS generates listening audio. All of it stays within the Gemini free tier (1,500 RPD); client details: ADR-031.
4. **Learning-language-neutral logic** — logic code never branches on a learning language or level name; language facts live in language-keyed data, so adding a language means adding data, not branches. Rules: [`docs/CONVENTIONS.md`](docs/CONVENTIONS.md#learning-language-neutrality) (ADR-081).

---

## 5. Coding conventions

When writing/changing code, migrations, or config, follow [`docs/CONVENTIONS.md`](docs/CONVENTIONS.md).

---

## 6. Reference docs

- [README.md](README.md) — project overview, tech stack, local dev/deploy
- [STATUS.md](STATUS.md) — current work state (🚨 read before working)
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) — system structure, data flow, callback convention
- [docs/adr/](docs/adr/) — technical decision records (`ADR_from_NN_to_MM.md` range files + separate files; rules: §3 Case A)
- [docs/workthrough/](docs/workthrough/) — incidents, verification evidence and procedures found nowhere else (criteria: §3 Case B Close)
- [docs/todos/](docs/todos/) — self-contained plan docs for TODOs executed in a separate session
- [Makefile](Makefile) — dev commands; its **header comment is the target manifest** — read that instead of the whole file
