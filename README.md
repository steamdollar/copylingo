# CopyLingo

CopyLingo is a personal language-learning automation app built around Telegram.

It manages seeded study materials and practice questions, delivers them through Telegram, grades user answers, and schedules review sessions with an SRS-style workflow. I use the current deployment for Japanese study, while the study model, seed format, and delivery flow are designed to support additional target languages. The project is both a real tool and a backend portfolio project focused on practical automation, data modeling, and service integration.

## What it does

Core flow:

```text
Seeded material/question records → Telegram push (Study/Quiz) → answer → grading → spaced review
```

Main capabilities:

- Seeds language-learning materials and questions from JSON records (`cmd/seeding`, data in `cmd/seeding/data/<lang>/<level>.json`)
- Covers vocabulary, script recognition, reading, handwriting, and listening questions
- Delivers Study cards and Quiz questions through a Telegram bot with inline interactions
- Supports Telegram Mini App based handwriting submissions
- Answer/handwriting grading, learning Q&A, and tip generation via an OpenAI-compatible endpoint (Gemini)
- Listening audio: Gemini native TTS, raw PCM transcoded to OGG/Opus with ffmpeg, cached in S3-compatible storage (content-addressed), Telegram file IDs reused
- Stores materials, questions, sessions, and review state in PostgreSQL; session/runtime state in Redis
- Produces structured application logs with interaction IDs for debugging

An NHK collection pipeline exists in `internal/pipeline` but is not wired (ADR-057).

## Why this project exists

The project is designed around two goals:

1. **Real personal use** — I use the current Japanese content as part of my daily study workflow, while the application model is not tied to a single target language.
2. **Backend engineering portfolio** — implementation choices are evaluated as if the product could grow beyond a single-user tool.

That means the project intentionally focuses on backend concerns such as data modeling, idempotent seeders, external API boundaries, logging, configuration, local infrastructure, and deployment reproducibility.

## Engineering highlights

- SQL-first PostgreSQL data access with sqlx, explicit queries, and versioned migrations
- Idempotent seeders for reproducible learning content
- SRS-based session building and scheduled review flows
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
          │     ├── grading, Q&A, tips
          │     └── native TTS
          ├── ffmpeg (PCM → OGG/Opus)
          └── MinIO / S3-compatible object storage
```

The Go server owns Telegram interaction handling, grading, review scheduling, and Mini App endpoints. Package layers and data flow: [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md).

## Tech stack

| Area | Technology | Notes |
|---|---|---|
| Language | Go 1.27 | Main backend application |
| HTTP framework | Gin | Health checks, admin/API endpoints, Mini App endpoints |
| Telegram | go-telegram-bot-api/v5 | Bot interactions and inline keyboard flows |
| Database | PostgreSQL 16 | SQL-first access with sqlx, explicit queries, and versioned migrations |
| Cache/runtime state | Redis 7 | Session/cache handling and runtime state |
| Configuration | Viper | YAML + environment variable override |
| Scheduler | robfig/cron/v3 | Batch jobs and scheduled learning flows |
| LLM runtime | Gemini | Grading, learning Q&A, and tips through an OpenAI-compatible chat endpoint |
| TTS | Gemini native TTS + ffmpeg | Pre-generated speech transcoded to OGG/Opus for Telegram |
| Object storage | MinIO / S3-compatible storage | Content-addressed speech audio cache |
| Infrastructure | Docker + Docker Compose | PostgreSQL, Redis, MinIO, and app runtime |

## Local development

The recommended local setup runs PostgreSQL, Redis, and MinIO through Docker while the Go server runs directly on the host machine. Host-based execution requires Go 1.27, the PostgreSQL client, and ffmpeg. The full Docker image already includes ffmpeg.

```bash
# 1. Configure the bot token and API key (see .env.example)
cp .env.example .env   # fill COPYLINGO_TELEGRAM_TOKEN and COPYLINGO_LLM_API_KEY

# 2. Start PostgreSQL, Redis, MinIO, and the audio bucket initializer
make infra

# 3. Seed the current study materials and questions
# Records live in cmd/seeding/data/<language>/<level>.json; ja is the only language today.
go run ./cmd/seeding ja

# 4. Run the Go server
go run ./cmd/server
```

A fresh DB volume is already initialized by the `migrations/` initdb mount (`docker-compose.yml`). Run `PGPASSWORD=copylingo make migrate` only for an existing volume.

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
| `COPYLINGO_LLM_API_KEY` | Gemini API key used for grading, tips, and native TTS |
| `COPYLINGO_LLM_MODEL` | Chat model override (grading, tips) |
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

Handwriting questions are submitted through a Telegram Mini App, which needs a public HTTPS URL. `make tunnel` starts a Cloudflare quick tunnel and writes the URL to `.env`. Then restart the server, or use `make tmux` to run everything. Register the tunnel host in BotFather.

Endpoints, security notes, and operations: [`docs/HANDWRITING_MINIAPP_INGRESS.md`](docs/HANDWRITING_MINIAPP_INGRESS.md)

## Deployment

Example deployment setup:

```bash
cat > .env <<'EOF'
COPYLINGO_TELEGRAM_TOKEN=<telegram-bot-token>
COPYLINGO_LLM_API_KEY=<gemini-api-key>
COPYLINGO_SERVER_PUBLIC_BASE_URL=https://copylingo.example.com
EOF

docker compose up -d

# Seed content from the host (the image builds only ./cmd/server)
go run ./cmd/seeding ja   # connects to localhost:5432
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

Design and log security rules: [Structured Logging in `docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md#structured-logging).

## Makefile

See the target manifest in the `Makefile` header comment.

## Project docs

- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — system architecture and data flow
- [`docs/adr/`](docs/adr/) — architecture decision records
- [`docs/CONVENTIONS.md`](docs/CONVENTIONS.md) — coding conventions
- [`AGENTS.md`](AGENTS.md) — project context and coding rules for agent-assisted development
- [`STATUS.md`](STATUS.md) — current work state and recently completed work

## Agent-assisted development workflow

For continuing work with a coding agent in a new session:

```text
Read AGENTS.md and STATUS.md, then continue
```

Agents record decisions in `docs/adr/` and work state in `STATUS.md` (see AGENTS.md §3).
