# CopyLingo — Coding Conventions

> Coding reference split out of [`AGENTS.md`](../AGENTS.md) §5. **Consult it when writing/changing code, migrations, or config** (Case B implementation). No need to read it for ADR discussion or docs-only work.

---

## Go code

1. **Package structure**: split by layer under `internal/` (`model`, `repository`, `service`, `bot`, `pipeline`, `external`).
2. **DB access**: write raw SQL with `sqlx`.
3. **Error handling**:
   - Don't log at the point of error; attach context and return with the `fmt.Errorf("context: %w", err)` pattern.
   - The repository layer includes searchable error context based on the function name / key identifiers (e.g. `SessionQuestionRepository.GetBySession session_id=%d: %w`).
   - The service layer wraps only when adding new business meaning. A plain repository pass-through returns the error as-is.
   - If `err` isn't reused afterward, narrow its scope with `if err := ...; err != nil` or `if _, err := ...; err != nil`.
4. **ID**: DB PKs are SERIAL (auto-increment). Only the `users` table uses the Telegram ID (BIGINT).
5. **Context**: every repository/service method takes `context.Context` as its first argument.
6. **Logging**:
   - Currently uses the standard `log` library (may switch to structured logging later).
   - Lower layers like the repository don't log directly.
   - Boundary layers — bot handlers, HTTP handlers, scheduler jobs — log once, with user/task context.
7. **Tests**: `*_test.go` files, located in the same package.

## Telegram bot

1. **Callback Data convention**: follow [`docs/ARCHITECTURE.md` "Callback Data 규약"](ARCHITECTURE.md#callback-data-규약) as the SSOT (not redefined here).
2. **Message format**: HTML parse mode (`ParseMode = "HTML"`).
3. **Keyboards**: use Inline Keyboards (not Reply Keyboards).

## DB

1. **Migrations**: this project does not accumulate migration SQL across multiple files — it keeps **only `migrations/001_init.sql`**. When the schema changes, merge it into `001_init.sql` instead of creating a new `002_*.sql`. `make migrate` can apply `NNN_*.sql` in order, but the operating rule is a single SQL file.
2. **Naming**: snake_case, plural table names (`users`, `questions`, `sessions`).
3. **Timestamp**: every table has `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`.
4. **JSONB**: use where a flexible structure is needed (questions.options, sessions.questions, article_responses.conversation).
5. **Indexes**: add only when needed. No standalone index on a low-cardinality column (boolean, enum, etc.).

## Config

- **Inject secrets via environment variables** (`COPYLINGO_TELEGRAM_TOKEN`, `COPYLINGO_LLM_API_KEY`, etc.). Never hardcode API keys/tokens in `config.yaml`.
