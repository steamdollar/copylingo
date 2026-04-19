# CopyLingo

> JLPT N1 달성을 목표로 하는 개인 일본어 학습 텔레그램 봇.

NHK Web Easy 크롤링 → AI 문제 생성 → 텔레그램 푸시 → 채점 → SRS 복습 파이프라인을 자동화한다.

---

## 🤖 에이전트 시작 가이드

**새 대화에서 작업을 이어가려면:**

```
"AGENTS.md 읽고 CURRENT_TASK.md 작업 이어줘"
```

| 파일 | 역할 |
|---|---|
| [`AGENTS.md`](AGENTS.md) | 프로젝트 컨텍스트, 코딩 규칙, 비즈니스 로직 |
| [`ROADMAP.md`](ROADMAP.md) | 전체 Phase/Subphase 진행 상황 |
| [`CURRENT_TASK.md`](CURRENT_TASK.md) | 현재/다음 작업 지시서 |

> [!NOTE]
> 에이전트 작업 완료 시 `CURRENT_TASK.md` → `ROADMAP.md` → `docs/HISTORY.md` 순으로 업데이트.

---

## 기술 스택

| 구성 요소 | 기술 |
|---|---|
| 백엔드 서버 | Go 1.25 + Gin |
| 인터페이스 | Telegram Bot API (Inline Keyboard) |
| DB | PostgreSQL 16 |
| 캐시 / 세션 | Redis 7 |
| 스케줄러 | robfig/cron/v3 |
| AI (문제 생성/대화) | Gemini 2.0 Flash (OpenAI 호환 엔드포인트) |
| TTS | Google Cloud TTS (사전 생성 + 파일 캐싱) |
| 배포 | Docker Compose |

---

## 인프라 구조

```
[Telegram] ←→ [Go 서버 :8080] ←→ [PostgreSQL :5432]
                                ←→ [Redis :6379]
                                ←→ [Gemini API (HTTPS)]
                                ←→ [NHK Web Easy RSS (크롤링)]
```

- 크롤링 및 외부 API 호출은 Go 서버가 직접 담당 (AI가 인터넷에 직접 접속하지 않음)

---

## 로컬 개발 환경 실행

인프라(DB + Redis)만 Docker로 띄우고, Go 서버는 호스트에서 직접 실행한다.

```bash
# 1. 인프라 기동 (PostgreSQL + Redis)
make infra

# 2. DB 마이그레이션 적용
make migrate

# 3. Go 서버 실행
COPYLINGO_TELEGRAM_TOKEN=<토큰> go run ./cmd/server
# 또는
make run
```

> `config.yaml`의 기본값이 `localhost:5432`, `localhost:6379`이므로 DB 설정 변경 불필요.

---

## 🚀 다른 머신에서 이어서 작업하기

1. **Repository Clone**: `git clone <repo_url>`
2. **Environment**: `.env.example`을 복사하여 `.env` 생성 후 API 키 설정.
3. **Infrastructure**: `make infra` (Docker Postgres/Redis 기동)
4. **Migration**: `make migrate` (DB 스키마 생성)
5. **Seeding**: `go run ./cmd/kana_seeder` (기초 가나 데이터 생성)

---

## AI 설정 (Gemini 무료 티어)

[Google AI Studio](https://aistudio.google.com)에서 API 키 발급 후 `config.yaml` 수정:

```yaml
openai:
  api_key: "AIza..."
  model: "gemini-2.0-flash"
  base_url: "https://generativelanguage.googleapis.com/v1beta/openai/"
```

| Gemini 2.0 Flash 무료 한도 | 예상 사용량 |
|---|---|
| 1,500 RPD | ~30회/일 (문제 생성 배치) |
| 15 RPM | 새벽 3시 배치, 무관 |

---

## 배포

```bash
# .env 파일 생성
echo "COPYLINGO_TELEGRAM_TOKEN=<토큰>" >> .env
echo "COPYLINGO_OPENAI_API_KEY=<Gemini API 키>" >> .env

# 전체 컨테이너 빌드 & 기동
docker compose up -d
```

기동 순서는 Compose healthcheck가 보장한다:
```
PostgreSQL (healthy) ──┐
                       ├──▶ Go 서버 기동
Redis      (healthy) ──┘
```

---

## Makefile

| 명령어 | 동작 |
|---|---|
| `make infra` | PostgreSQL + Redis 컨테이너만 기동 |
| `make run` | Go 서버 로컬 실행 |
| `make build` | 바이너리 빌드 (`bin/copylingo`) |
| `make migrate` | DB 마이그레이션 적용 |
| `make docker-up` | 전체 컨테이너 기동 (배포용) |
| `make docker-down` | 전체 종료 |
| `make test` | 테스트 실행 |

---

## 콘텐츠 수집 대상

| 사이트 | 용도 | 수집 방식 | JLPT 난이도 |
|---|---|---|---|
| NHK Web Easy | 읽기 소재 | RSS 피드 | N4~N3 |
| NHK ニュース | 읽기 소재 | RSS 피드 | N2~N1 |
| Tanos JLPT | 어휘/문법 seed | HTML GET | N1~N5 |
| JLPT Sensei | 문법 패턴 | HTML GET | N1~N5 |
| jlpt.jp 공식 | 샘플 문제 | HTML GET | N1~N5 |

---

## 문서

- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — 시스템 아키텍처, 데이터 흐름
- [`docs/ADR.md`](docs/ADR.md) — 기술 의사결정 기록
- [`docs/HISTORY.md`](docs/HISTORY.md) — 개발 히스토리
