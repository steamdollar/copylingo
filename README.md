# CopyLingo

CopyLingo is a personal Japanese learning automation app built around Telegram.

It collects Japanese learning material, generates practice exercises, delivers them through Telegram, grades user answers, and schedules review sessions with an SRS-style workflow. The project is both a real tool I use for my own study and a backend portfolio project focused on practical automation, data modeling, and service integration.

## What it does

Core flow:

```text
Content collection → exercise generation → Telegram delivery → answer submission → grading → spaced review
```

Main capabilities:

- Collects reading material and JLPT-oriented study data from external sources
- Generates Japanese practice exercises for vocabulary, kana, reading, and handwriting flows
- Delivers questions through a Telegram bot with inline interactions
- Supports Telegram Mini App based handwriting submissions
- Stores learning materials, questions, sessions, and review state in PostgreSQL
- Uses Redis for session/cache-related runtime state
- Produces structured application logs with interaction IDs for debugging

## Why this project exists

The project is designed around two goals:

1. **Real personal use** — I use it as part of my Japanese study workflow.
2. **Backend engineering portfolio** — implementation choices are evaluated as if the product could grow beyond a single-user tool.

That means the project intentionally focuses on backend concerns such as data modeling, idempotent seeders, external API boundaries, logging, configuration, local infrastructure, and deployment reproducibility.

## Architecture

```text
[Telegram Bot / Mini App]
          ↓
[Go server :8080]
          ├── PostgreSQL :5432
          ├── Redis :6379
          ├── Gemini API
          ├── Google Cloud TTS
          └── External content sources
```

The Go server owns crawling, API calls, question generation orchestration, Telegram interaction handling, grading, and review scheduling.

## Tech stack

| Area | Technology | Notes |
|---|---|---|
| Language | Go 1.25 | Main backend application |
| HTTP framework | Gin | Health checks, admin/API endpoints, Mini App endpoints |
| Telegram | go-telegram-bot-api/v5 | Bot interactions and inline keyboard flows |
| Database | PostgreSQL 16 | sqlx, raw SQL, ORM-free approach |
| Cache/runtime state | Redis 7 | Session/cache handling and runtime state |
| Configuration | Viper | YAML + environment variable override |
| Scheduler | robfig/cron/v3 | Batch jobs and scheduled learning flows |
| LLM runtime | Gemini | Exercise generation through an OpenAI-compatible endpoint |
| TTS | Google Cloud TTS | Pre-generated and cached speech audio |
| Infrastructure | Docker + Docker Compose | PostgreSQL, Redis, and app runtime |

## Local development

The recommended local setup runs PostgreSQL and Redis through Docker, while the Go server runs directly on the host machine.

```bash
# 1. Start local infrastructure
make infra

# 2. Apply database migrations
make migrate

# 3. Seed study materials and questions
go run ./cmd/ja/material_seeder
go run ./cmd/ja/kana_seeder
go run ./cmd/ja/vocab_seeder

# 4. Run the Go server
COPYLINGO_TELEGRAM_TOKEN="<telegram-bot-token>" \
COPYLINGO_LLM_API_KEY="<llm-api-key>" \
go run ./cmd/server
```

Or use:

```bash
make run
```

`config.yaml` defaults to local PostgreSQL and Redis endpoints:

```text
localhost:5432
localhost:6379
```

## Required environment variables

| Variable | Purpose |
|---|---|
| `COPYLINGO_TELEGRAM_TOKEN` | Telegram bot token |
| `COPYLINGO_LLM_API_KEY` | LLM provider API key |
| `COPYLINGO_LLM_MODEL` | LLM model name override |
| `COPYLINGO_SERVER_PUBLIC_BASE_URL` | Public HTTPS base URL for Telegram Mini App flows |

For local Mini App testing, `COPYLINGO_SERVER_PUBLIC_BASE_URL` must point to a public HTTPS URL because mobile Telegram cannot access your machine's `localhost`.

## Telegram Mini App + Cloudflare Tunnel

Handwriting questions are submitted through a Telegram Mini App. This requires an externally reachable HTTPS URL.

Current Mini App endpoints:

- `GET /miniapp/handwriting`
- `POST /api/miniapp/handwriting/submit`

Local test flow:

```bash
export COPYLINGO_TELEGRAM_TOKEN="<telegram-bot-token>"
export COPYLINGO_LLM_API_KEY="<llm-api-key>"

make infra
make migrate
go run ./cmd/ja/kana_seeder
go run ./cmd/server
```

Start a Cloudflare Tunnel:

```bash
make tunnel
```

Then set the public base URL:

```bash
export COPYLINGO_SERVER_PUBLIC_BASE_URL="https://xxxxx.trycloudflare.com"
go run ./cmd/server
```

Required checks:

- Register the Mini App/Web App domain in BotFather.
- Ensure the `public_base_url` host matches the registered Telegram domain.
- Restart the server when the tunnel URL changes.

More detail: [`docs/HANDWRITING_MINIAPP_INGRESS.md`](docs/HANDWRITING_MINIAPP_INGRESS.md)

## Deployment

Example deployment setup:

```bash
cat > .env <<'EOF'
COPYLINGO_TELEGRAM_TOKEN=<telegram-bot-token>
COPYLINGO_LLM_API_KEY=<llm-api-key>
COPYLINGO_SERVER_PUBLIC_BASE_URL=https://copylingo.example.com
EOF

docker compose up -d
```

Compose startup order is guarded by health checks:

```text
PostgreSQL (healthy) ──┐
                       ├──▶ Go server
Redis      (healthy) ──┘
```

## Logging

Application logs are written to stdout and to daily JSONL files:

```text
./logs/copylingo-YYYY-MM-DD.jsonl
```

The default log timezone is `Asia/Seoul`, and daily log files older than the retention window are removed automatically.

```bash
# Tail today's logs
tail -f logs/copylingo-$(date +%F).jsonl | jq

# Filter error logs
jq 'select(.level == "ERROR")' logs/copylingo-2026-06-01.jsonl

# Trace a single Telegram update or request
jq 'select(.interaction_id == "tg-12345")' logs/copylingo-2026-06-01.jsonl
```

Logging configuration:

| Variable | Default |
|---|---|
| `COPYLINGO_LOGGING_DIR` | `./logs` |
| `COPYLINGO_LOGGING_LEVEL` | `INFO` |
| `COPYLINGO_LOGGING_RETENTION_DAYS` | `30` |
| `COPYLINGO_LOGGING_TIMEZONE` | `Asia/Seoul` |

Security note: tokens, Telegram `init_data`, raw user answers, and handwriting stroke coordinates are not written to logs.

## Makefile

| Command | Description |
|---|---|
| `make infra` | Start PostgreSQL and Redis containers only |
| `make run` | Run the Go server locally |
| `make build` | Build binary to `bin/copylingo` |
| `make migrate` | Apply database migrations |
| `make docker-up` | Start the full Docker Compose stack |
| `make docker-down` | Stop the full Docker Compose stack |
| `make test` | Run tests |

## Content sources

| Source | Purpose | Collection method | JLPT level |
|---|---|---|---|
| NHK Web Easy | Reading material | RSS feed | N4~N3 |
| NHK News | Reading material | RSS feed | N2~N1 |
| Tanos JLPT | Vocabulary / grammar seed data | HTML GET | N1~N5 |
| JLPT Sensei | Grammar patterns | HTML GET | N1~N5 |
| jlpt.jp official site | Sample questions | HTML GET | N1~N5 |

## Project docs

- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — system architecture and data flow
- [`docs/ADR.md`](docs/ADR.md) — architecture decision records
- [`docs/HISTORY.md`](docs/HISTORY.md) — development history
- [`AGENTS.md`](AGENTS.md) — project context and coding rules for agent-assisted development
- [`ROADMAP.md`](ROADMAP.md) — project roadmap and phase tracking
- [`CURRENT_TASK.md`](CURRENT_TASK.md) — current work item and next implementation target

## Agent-assisted development workflow

For continuing work with a coding agent in a new session:

```text
Read AGENTS.md and continue from CURRENT_TASK.md
```

When an agent finishes a task, update documents in this order:

```text
CURRENT_TASK.md → ROADMAP.md → docs/HISTORY.md
```
