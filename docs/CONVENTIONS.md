# CopyLingo — Coding Conventions

> Coding reference split out of [`AGENTS.md`](../AGENTS.md) §5. **Consult it when writing/changing code, migrations, or config** (Case B implementation). No need to read it for ADR discussion or docs-only work.

---

## Go code

1. **Package structure**: split by responsibility under `internal/`. Allowed imports between packages are pinned by `internal/import_boundary_test.go` (see [ARCHITECTURE.md](ARCHITECTURE.md)).
2. **DB access**: write raw SQL with `sqlx`.
3. **Error handling**:
   - Don't log at the point of error; attach context and return with the `fmt.Errorf("context: %w", err)` pattern.
   - The repository layer includes searchable error context based on the function name / key identifiers (e.g. `QuestionRepository.UpsertSeedBatch count=%d batch_size=%d: %w`).
   - The service layer wraps only when adding new business meaning. A plain repository pass-through returns the error as-is.
   - If `err` isn't reused afterward, narrow its scope with `if err := ...; err != nil` or `if _, err := ...; err != nil`.
4. **ID**: DB PKs are SERIAL (auto-increment). Only the `users` table uses the Telegram ID (BIGINT).
5. **Context**: every repository/service method takes `context.Context` as its first argument.
6. **Logging**:
   - Use `log/slog` (JSON handler, `internal/observability`). Request attributes such as `interaction_id` come from the context.
   - Lower layers like the repository don't log directly.
   - Boundary layers — bot handlers, HTTP handlers, scheduler jobs — log once, with user/task context.
7. **Tests**: `*_test.go` files, located in the same package.
8. **Parameter layout**: 함수 매개변수와 호출 인자가 2개 이상이면 줄 길이와 관계없이 각각 별도 줄에 쓴다. 반환값 목록은 기존 스타일을 유지한다. VS Code에서는 `cmd/dev/goparams`를 custom formatter로 연결해 저장 시 적용한다. 기존 gofmt/golines는 이 개행을 유지한다.

### Agent Go formatting

Main agent·subagent 모두 Go 파일 작성·수정 후, 테스트와 인계/완료 보고 전에 VS Code와 같은 `goparams`를 적용한다. 파일 도구나 셸로 코드를 쓰는 동작은 VS Code 저장 시 포맷을 실행하지 않는다.

- **대상**: 이번 작업에서 직접 생성·수정한 `.go` 파일만 명시한다. 다른 작업자의 변경까지 포함하는 전체 dirty 파일 목록이나 저장소 전체를 일괄 포맷하지 않는다.
- **순서**: 코드 수정 → 필요한 import 정리·기존 포맷 → `goparams` → `make test` 및 최종 diff 확인. 이후 Go 코드를 다시 수정하면 해당 파일에 재적용한다. 포맷 실패는 해결하거나 미완료로 보고하며 사용자 저장에 맡기지 않는다.
- **실행**: 프로젝트 루트에서 최신 소스로 도구를 빌드한다. 아래 경로를 실제 수정 파일 목록으로 바꾼다. stdout 전체를 정상적으로 받은 뒤에만 원본을 쓰므로 실패 시 원본이 잘리지 않는다. `goparams < file.go > file.go`처럼 입력과 출력을 같은 파일로 리다이렉트하지 않는다.

```bash
go build -o bin/goparams ./cmd/dev/goparams &&
python3 - path/to/changed.go path/to/another.go <<'PY'
import subprocess
import sys
from pathlib import Path

for filename in sys.argv[1:]:
    path = Path(filename)
    formatted = subprocess.check_output(["./bin/goparams"], input=path.read_bytes())
    path.write_bytes(formatted)
PY
```

Go 파일을 변경하지 않은 문서 전용 작업은 이 단계를 생략한다.

## Telegram bot

1. **Callback Data convention**: follow [`docs/ARCHITECTURE.md` "Callback Data 규약"](ARCHITECTURE.md#callback-data-규약) as the SSOT (not redefined here).
2. **Message format**: HTML parse mode (`ParseMode = "HTML"`).
3. **Keyboards**: use Inline Keyboards (not Reply Keyboards).

## DB

1. **Migrations**: this project does not accumulate migration SQL across multiple files — it keeps **only `migrations/001_init.sql`**. When the schema changes, merge it into `001_init.sql` instead of creating a new `002_*.sql`. `make migrate` can apply `NNN_*.sql` in order, but the operating rule is a single SQL file.
2. **Naming**: snake_case, plural table names (`users`, `questions`, `sessions`).
3. **Timestamp**: new tables get `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`.
4. **JSONB**: use where a flexible structure is needed (`questions.options`, `materials.payload`).
5. **Indexes**: add only when needed. No standalone index on a low-cardinality column (boolean, enum, etc.).

## Config

- **Inject secrets via environment variables** (`COPYLINGO_TELEGRAM_TOKEN`, `COPYLINGO_LLM_API_KEY`, etc.). Never hardcode API keys/tokens in `config.yaml`.
