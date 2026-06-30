# CopyLingo — Agent Contract & Doc Router

> Shared rules every AI agent must follow when working on CopyLingo. **Codex auto-loads this file directly**; **Claude / Gemini point to it from their own files (`CLAUDE.md`, `GEMINI.md`).** So this is the common entry **contract** for every agent — the SSOT for roles, work protocol, and project design criteria, and a **router** to detailed rules (coding conventions, architecture, delegation, etc. — the §6 satellite docs). Don't copy detail into this file; point to the owning doc.

---

## 1. Agent entry rules

Each CLI auto-loads its own convention file. This mapping is tool-side behavior and can't be changed.

| agent | auto-loaded file | how it reaches AGENTS.md |
|---|---|---|
| Claude Code | `CLAUDE.md` | delegated from `CLAUDE.md`'s first line |
| Codex | `AGENTS.md` | loads this file directly |
| Gemini CLI | `GEMINI.md` | delegated from `GEMINI.md`'s first line |

Each per-agent file is a **thin overlay on top of this document**; shared rules live here and in the §6 satellite docs.

**Precedence over personal config**: when an agent's personal global config (e.g. Claude's `~/.claude/*`) conflicts with this contract, **this contract wins** (project work protocol, coding conventions, delegation topology, etc.). Non-conflicting personal workflow (output language, tone, review procedure, per-turn reporting format, etc.) still applies.

---

## 2. Agent role matrix

| actor | entry | responsibility |
|---|---|---|
| **User** | start of every session | **Final decision-maker.** Final approval on every decision (design, implementation direction, ADR adoption, TODO delegation, etc.). Agents propose, execute, and review, but **must get user confirmation on any non-trivial decision**. |
| **Claude Code / Codex** (main agent) | user starts the session directly | Carries one task end to end — **design · implement · verify · review**. The two are **chosen by user preference** (record any meaningful capability difference in this doc). Escalates non-trivial judgment to the user. |
| **native subagent** | native-spawned by the main agent | **Default delegation mechanism.** Runs bounded, parallelizable subtasks. Bulk mechanical work (large reads/classification/summarization) is **overridden down to the Haiku tier**. The main agent re-verifies results. |
| **external executor** (Gemini=agy) | main agent dispatches via a self-contained TODO doc | **Special quota-isolation executor.** For high-volume generation batches only (estimated tokens > threshold). Executes to spec; stops and asks at decision points. |

> **Native subagent is the default for delegation** — follow [`docs/NATIVE_SUBAGENT_DELEGATION.md`](docs/NATIVE_SUBAGENT_DELEGATION.md). The external Gemini CLI (=agy) is a **high-volume-generation escape valve** that isolates free quota; dispatch to it only via a self-contained TODO doc (dispatch rules [`GEMINI_CLI_DELEGATION.md`](docs/GEMINI_CLI_DELEGATION.md), executor contract [`GEMINI_CLI_EXECUTION.md`](docs/GEMINI_CLI_EXECUTION.md)). **Never send a task that isn't self-contained**, either way.
>
> **ROI gate**: delegation is a token-saving tool, not a default. Size the work with a cheap local scan (`rg`/`git diff`/`go test`) first; if candidates are few or the task is mechanical, the main agent does it directly, and re-verifies any subagent result as the final owner. Switch to the external Gemini only on ① a user conserve directive or ② **estimated batch tokens > ~50k**, reporting the estimate when you do (the agent can't query Max-plan usage, so never self-judge "usage is tight"). Full criteria: [`docs/NATIVE_SUBAGENT_DELEGATION.md`](docs/NATIVE_SUBAGENT_DELEGATION.md) "Delegation ROI Gate / Executor Selection".

### Role substitution / fallback

> The role matrix is a default, not a permission boundary. If the user assigns the work differently, that agent does it — but the §3 work protocol applies unchanged.

---

## 3. Work protocol — Case classification

A user request falls into one of these, and the deliverable and procedure differ by class. **Most requests start at Case 0 and explicitly escalate to A/B/C as needed.**

> **Pre-branch — micro-change: the user handles it directly**
>
> For a **very small change** (e.g. one character, one variable name, a single typo) that's inefficient in agent tokens/time, don't do it — say **"It's more token-efficient to fix this yourself."** Only proceed if the user insists.

### Case 0. Investigate / review / answer (default)

> Answering a question, or analyzing/reviewing/explaining code, queries, design, or docs **without changing** code or docs. Most real requests start here.

- **Owner**: Claude / Codex
- **Procedure**: confirm facts with as much search/read as needed **before answering** (no guessing). No separate deliverable (doc/code).
- **Escalation**: if investigation surfaces a need, **switch explicitly** to that Case (no implicit changes):
  - non-trivial design decision → Case A
  - code/migration/config change → Case B
  - improvement outside current scope (not handled now) → Case C

### Case A. Decision (ADR)

> Discussing a **decision about how the codebase is built** — system architecture, implementation approach, tradeoffs, etc.

- **Owner**: Claude / Codex
- **Procedure**:
  1. Discuss with the user thoroughly. State tradeoffs at the assumed scale (see §4).
  2. **Once the decision settles, the agent immediately adds an entry to the latest ADR file under `docs/adr/` (currently `ADR_from_21_to_40.md`) without waiting to be asked** (background / decision / consequences). The user often forgets to update ADRs, so do it proactively.
  3. If code changes follow, continue into Case B.
- **Note**: don't default to "it's fine, single user" or "YAGNI" — rationale and application are in §4.

### Case B. Writing code

> Once scope is set, handling **plan → implement → verify → close** in one flow.

- **Owner**: Claude / Codex (user's choice)
- **Procedure**:
  1. **Start**: check "🔨 In progress" items in `STATUS.md` — judge relevance to the current request.
  2. **Plan**: for non-trivial work, agree the plan with the user before implementing.
  3. **Implement**: follow the `internal/` layer structure and coding conventions (§5).
  4. **Verify**: `make test` is **required** for code/migration/config changes. For docs-only work, record the skip reason in the workthrough.
     - For changes that must take effect in the local runtime (Go server, Mini App static assets, config), after verifying, **consult the [`Makefile`](Makefile) target manifest (header comment) to pick the right restart target** and restart the relevant instance — e.g. App with `make restart-app`, then confirm `http://localhost:8080/health`.
     - Restart DB/Redis/Tunnel only when you changed that component directly (`make restart-db` / `make restart-redis`).
  5. **Close**:
     - Update `STATUS.md` — move "in progress" → "📝 recently done" **only when the current request completes the in-progress item itself**. A side task unrelated to the in-progress item (e.g. doc cleanup, handling an incidental finding) either leaves STATUS.md alone or adds a single line under "📝 recently done".
     - For non-trivial work, create `docs/workthrough/YYMMDDhhmm_<job>.md` — changed files, decisions, verification results.
     - If a decision was made, update the latest ADR file under `docs/adr/`.
     - Update `ROADMAP.md` only on milestone completion.
- **Language**: implementation plans and workthroughs are written in **Korean**.

### Case C. Splitting off and delegating a TODO

> During work you **find an anomaly/improvement outside current scope** and only record it instead of handling it now.

- **Trigger**: the user or agent finds an issue outside current scope during review/implementation, but judges that handling it now would blow up the scope, so decides to leave it as a TODO.
- **Owners**:
  - **Split decision**: user
  - **Doc authoring**: the current main session's agent
  - **Execution**: the owner agent in a separate session

#### Splitting (main agent)

1. **Always write `docs/todos/<task>.md` as a self-contained plan doc** (no one-line-summary escape hatch). Include:
   - background/purpose
   - list of files to change + before/after snippets
   - verification method (`make test`, etc.)
   - off-limits areas, decisions already made
2. If an ambiguous decision point comes up while planning, **settle it with the user now and pin it in the doc** (so the executing agent doesn't have to re-ask).
3. Register a one-line summary + doc link in `STATUS.md`:
   `- [ ] <one-line summary> — see [docs/todos/<file>.md](docs/todos/<file>.md)`

#### Execution (owner agent in a separate session — main agent or external Gemini=agy)

1. Find the plan path in `STATUS.md` → read `docs/todos/<task>.md` carefully. If clear, start immediately and **follow Case B steps 3 (implement) → 4 (verify) → 5 (close) as-is**; if any judgment beyond what the plan pins is needed, stop and ask the user. (An external Gemini CLI executor follows the [`docs/GEMINI_CLI_EXECUTION.md`](docs/GEMINI_CLI_EXECUTION.md) contract.)
2. **Extra handling at Case C close** (on top of Case B close):
   - remove that TODO's checkbox item from `STATUS.md` (separate from Case B's "in progress → recently done" rule)
   - delete `docs/todos/<task>.md` (preserved in git history)

### Transitions between cases

Cases often change mid-work. Handle all of these explicitly (no implicit case changes):

| transition | trigger | handling |
|---|---|---|
| **0 → A/B/C** | investigation/review reveals a need to decide, implement, or split a TODO | explicit escalation into that Case's procedure (A=design decision, B=code change, C=out-of-scope improvement) |
| **A → B** | code changes follow an ADR decision | record the ADR entry, then enter Case B's procedure immediately |
| **B → A** | a new **non-trivial decision** affecting assumed scale/architecture appears mid-implementation | **pause implementation** → agree with the user via Case A → record the ADR immediately → return to Case B |
| **B → C** | an anomaly/improvement outside scope is found | don't stop the current implementation; just split a TODO via Case C → resume the original Case B afterward |
| **C → B** | execution needs a non-trivial decision the plan didn't specify, or the scope turns out far larger than the plan | the executing agent stops immediately and reports to the user → the user decides to either augment the plan and resume, or promote it to a Case B in the main session |

---

## 4. Project character & design criteria ⚠️

> **Every architecture/refactor decision in this project is judged against this section.**

- CopyLingo is a dual-purpose project — **(a) real foreign-language learning + (b) portfolio** — and **when coding, (b) takes priority**.
- There is only 1 real user, but architecture/refactor decisions are judged **assuming this system serves tens of thousands of users**.
- Why: gaining experience handling larger-scale systems is one of this project's core purposes.

### How to apply

- **Default assumption is scale**: don't lay down "it's fine, single user" or "YAGNI" as a default *without basis*. But this doesn't mean "always pick the more complex pattern" — it means **right-size under the scale assumption and state that judgment**.
- Patterns that genuinely matter at the assumed scale (cache SSOT, event stream/outbox, CQRS, async workers, etc.) are chosen **deliberately, with tradeoffs**. **No cargo-cult** — pinning patterns in without load justification; this is not "distributed systems everywhere".
- **Deciding to "add" complexity and deciding "not to" are both first-class judgments.** The portfolio signal is *right-sizing with explicit tradeoffs*, not complexity itself; a non-trivial decision not to add something is also ADR-worthy.
- Design docs (`docs/adr/`, etc.) are **first-class deliverables alongside code**. Update the ADR together with any non-trivial design change.

### Decision principles

1. **Single-user optimization applies only to feature scope** — no social features, no monetization nudges (hearts, etc.). **Never apply it to architecture/performance decisions.**
2. **Push-based learning** — the bot sends sessions first; the user doesn't come to it.
3. **Heavy AI use** — problem generation, article conversations, and feedback are all AI-based.
4. **AI model operation** — Gemini in OpenAI-compatible mode (within the free 1,500 RPD/month).

---

## 5. Coding conventions

> When writing/changing code/migrations/config, follow [`docs/CONVENTIONS.md`](docs/CONVENTIONS.md) — Go (package structure, error handling, logging, testing), Telegram bot (callback convention, message format, keyboards), DB (migrations, naming, JSONB, indexes), config (env-var injection). No need to read it for ADR discussion or docs-only work.

---

## 6. Reference docs

- [README.md](README.md) — project overview, tech stack, local dev/deploy
- [STATUS.md](STATUS.md) — current work state (🚨 read before working)
- [ROADMAP.md](ROADMAP.md) — overall Phase/Subphase progress
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) — system structure, data flow, callback convention
- [docs/adr/](docs/adr/) — technical decision records (split by range: `ADR_from_01_to_20.md`, `ADR_from_21_to_40.md`)
- [docs/CONVENTIONS.md](docs/CONVENTIONS.md) — coding conventions (Go / Telegram bot / DB / config)
- [docs/workthrough/](docs/workthrough/) — detailed records of completed work
- [docs/todos/](docs/todos/) — self-contained plan docs for TODOs executed in a separate session
- [docs/NATIVE_SUBAGENT_DELEGATION.md](docs/NATIVE_SUBAGENT_DELEGATION.md) — runtime native child-agent spawn protocol
- [docs/GEMINI_CLI_DELEGATION.md](docs/GEMINI_CLI_DELEGATION.md) — Gemini CLI external delegation, retry, recovery protocol
- [docs/GEMINI_CLI_EXECUTION.md](docs/GEMINI_CLI_EXECUTION.md) — minimal execution contract read by an invoked Gemini CLI executor
- [Makefile](Makefile) — dev commands (`make test`, `make infra`, `make migrate`, `make build`, etc.); its **header comment is a target manifest** — read that instead of the whole file, and consult it to pick a restart target after a runtime-affecting change. Also tabulated in README.md's "Makefile" section
