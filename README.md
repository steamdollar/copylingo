# CopyLingo

CopyLingo is a personal language-learning automation app built around Telegram.

It manages study materials, generates practice exercises, delivers them through Telegram, grades user answers, and schedules review sessions with an SRS-style workflow. I use the current deployment for Japanese study, while the study model, content pipeline, and delivery flow are designed to support additional target languages. The project is both a real tool and a backend portfolio project focused on practical automation, data modeling, and service integration.

## What it does

Core flow:

```text
Study material → exercise generation → Telegram delivery → answer submission → grading → spaced review
```

Main capabilities:

- Manages seeded language-learning materials and generated practice questions
- Generates exercises for vocabulary, script recognition, reading, handwriting, and listening
- Delivers questions through a Telegram bot with inline interactions
- Supports Telegram Mini App based handwriting submissions
- Generates listening audio with Gemini native TTS and transcodes raw PCM into Telegram-ready OGG/Opus
- Caches speech audio in S3-compatible object storage and reuses Telegram file IDs
- Stores learning materials, questions, sessions, and review state in PostgreSQL
- Uses Redis for session/cache-related runtime state
- Produces structured application logs with interaction IDs for debugging

Planned content ingestion from external reading and language-proficiency sources is tracked separately in the roadmap and project documents.

## Why this project exists

The project is designed around two goals:

1. **Real personal use** — I use the current Japanese content as part of my daily study workflow, while the application model is not tied to a single target language.
2. **Backend engineering portfolio** — implementation choices are evaluated as if the product could grow beyond a single-user tool.

That means the project intentionally focuses on backend concerns such as data modeling, idempotent seeders, external API boundaries, logging, configuration, local infrastructure, and deployment reproducibility.

## Engineering highlights

- SQL-first PostgreSQL data access with sqlx, explicit queries, and versioned migrations
- Idempotent seeders for reproducible learning-content generation
- SRS-based session building and scheduled review flows
- Gemini-powered exercise generation and native speech synthesis
- PCM-to-OGG/Opus transcoding for Telegram voice delivery
- Content-addressed audio caching in S3-compatible object storage
- Telegram file ID reuse to avoid redundant audio uploads
- Telegram Mini App validation, session ownership checks, and server-side handwriting grading
- Structured JSON logging with interaction IDs across HTTP, Telegram updates, and scheduled jobs

## Architecture

```text
[Telegram Bot / Mini App]
          ↓
[Go server :8080]
          ├── PostgreSQL :5432
          ├── Redis :6379
          ├── Gemini API
          │     ├── exercise generation
          │     └── native TTS
          ├── ffmpeg (PCM → OGG/Opus)
          └── MinIO / S3-compatible object storage
```

The Go server owns question generation orchestration, Telegram interaction handling, grading, review scheduling, Mini App endpoints, and supporting API calls. For listening exercises, it generates speech with Gemini native TTS, transcodes the audio with ffmpeg, stores it in S3-compatible object storage, and reuses cached Telegram file IDs.

## Tech stack

| Area | Technology | Notes |
|---|---|---|
| Language | Go 1.25 | Main backend application |
| HTTP framework | Gin | Health checks, admin/API endpoints, Mini App endpoints |
| Telegram | go-telegram-bot-api/v5 | Bot interactions and inline keyboard flows |
| Database | PostgreSQL 16 | SQL-first access with sqlx, explicit queries, and versioned migrations |
| Cache/runtime state | Redis 7 | Session/cache handling and runtime state |
| Configuration | Viper | YAML + environment variable override |
| Scheduler | robfig/cron/v3 | Batch jobs and scheduled learning flows |
| LLM runtime | Gemini | Exercise generation through an OpenAI-compatible chat endpoint |
| TTS | Gemini native TTS + ffmpeg | Pre-generated speech transcoded to OGG/Opus for Telegram |
| Object storage | MinIO / S3-compatible storage | Content-addressed speech audio cache |
| Infrastructure | Docker + Docker Compose | PostgreSQL, Redis, MinIO, and app runtime |

## Local development

The recommended local setup runs PostgreSQL, Redis, and MinIO through Docker while the Go server runs directly on the host machine. Host-based execution requires Go 1.25, the PostgreSQL client, and ffmpeg. The full Docker image already includes ffmpeg.

```bash
# 1. Start PostgreSQL, Redis, MinIO, and the audio bucket initializer
make infra

# 2. Apply database migrations
make migrate

# 3. Seed the current study materials and questions
# The current seed package contains Japanese learning content.
go run ./cmd/ja/seeder

# 4. Run the Go server
COPYLINGO_TELEGRAM_TOKEN="<telegram-bot-token>" \
COPYLINGO_LLM_API_KEY="<gemini-api-key>" \
go run ./cmd/server
```

Or use:

```bash
make run
```

`config.yaml` provides local defaults for the infrastructure started by `make infra`:

```text
PostgreSQL  localhost:5432
Redis       localhost:6379
MinIO       http://localhost:9000
Bucket      copylingo-audio
```

## Runtime configuration

Core variables:

| Variable | Purpose |
|---|---|
| `COPYLINGO_TELEGRAM_TOKEN` | Telegram bot token |
| `COPYLINGO_LLM_API_KEY` | Gemini API key used for exercise generation and native TTS |
| `COPYLINGO_LLM_MODEL` | Exercise-generation model override |
| `COPYLINGO_SERVER_PUBLIC_BASE_URL` | Public HTTPS base URL required for Telegram Mini App flows |

Object storage can use the local MinIO defaults from `config.yaml` or be overridden for another S3-compatible service:

| Variable | Purpose |
|---|---|
| `COPYLINGO_STORAGE_ENDPOINT` | S3-compatible endpoint; leave empty for the AWS S3 default |
| `COPYLINGO_STORAGE_REGION` | Object-storage region |
| `COPYLINGO_STORAGE_BUCKET` | Speech-audio bucket name |
| `COPYLINGO_STORAGE_ACCESS_KEY` | Object-storage access key |
| `COPYLINGO_STORAGE_SECRET_KEY` | Object-storage secret key |
| `COPYLINGO_STORAGE_USE_PATH_STYLE` | Enables path-style addressing for MinIO-compatible services |

Gemini native TTS reuses `COPYLINGO_LLM_API_KEY`; it does not require separate Google Cloud TTS credentials.

For local Mini App testing, `COPYLINGO_SERVER_PUBLIC_BASE_URL` must point to a public HTTPS URL because mobile Telegram cannot access your machine's `localhost`.

## Telegram Mini App + Cloudflare Tunnel

Handwriting questions are submitted through a Telegram Mini App. This requires an externally reachable HTTPS URL.

Current Mini App endpoints:

- `GET /miniapp/handwriting`
- `POST /api/miniapp/handwriting/submit`

Local test flow:

```bash
export COPYLINGO_TELEGRAM_TOKEN="<telegram-bot-token>"
export COPYLINGO_LLM_API_KEY="<gemini-api-key>"

make infra
make migrate
go run ./cmd/ja/seeder
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
COPYLINGO_LLM_API_KEY=<gemini-api-key>
COPYLINGO_SERVER_PUBLIC_BASE_URL=https://copylingo.example.com
EOF

docker compose up -d
```

Compose startup is guarded by health checks for the stateful dependencies, and a one-shot MinIO job creates the audio bucket:

```text
PostgreSQL (healthy) ──┐
Redis      (healthy) ──┼──▶ Go server
MinIO      (healthy) ──┘
          └──────────────▶ minio-createbucket (one-shot)
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
| `make infra` | Start PostgreSQL, Redis, MinIO, and the audio bucket initializer |
| `make run` | Run the Go server locally |
| `make build` | Build binary to `bin/copylingo` |
| `make migrate` | Apply database migrations |
| `make docker-up` | Start the full Docker Compose stack |
| `make docker-down` | Stop the full Docker Compose stack |
| `make test` | Run tests |

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