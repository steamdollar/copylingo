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
2. **Environment**: 필요한 환경변수 직접 설정
3. **Infrastructure**: `make infra` (Docker Postgres/Redis 기동)
4. **Migration**: `make migrate` (DB 스키마 생성)
5. **Seeding**: `go run ./cmd/kana_seeder` (기초 가나 + 손글씨 문항 생성)
6. **Run**: `COPYLINGO_TELEGRAM_TOKEN=... COPYLINGO_LLM_API_KEY=... go run ./cmd/server`

예시:

```bash
export COPYLINGO_TELEGRAM_TOKEN="<telegram-bot-token>"
export COPYLINGO_LLM_API_KEY="<gemini-api-key>"

make infra
make migrate
go run ./cmd/kana_seeder
go run ./cmd/server
```

## Telegram Mini App + Cloudflare Tunnel 설정

손글씨 가나 문항은 Telegram Mini App으로 열리므로, 로컬 서버를 단순 `localhost:8080`으로만 띄워서는 휴대폰 Telegram 앱에서 접근할 수 없습니다.
다른 머신에서도 동일하게, **외부에서 접근 가능한 HTTPS URL**을 만든 뒤 그 URL을 `COPYLINGO_SERVER_PUBLIC_BASE_URL`로 주입해야 합니다.

현재 손글씨 플로우는 아래 엔드포인트를 사용합니다.

- `GET /miniapp/handwriting`
- `POST /api/miniapp/handwriting/submit`

### 1. 로컬 서버 실행

```bash
export COPYLINGO_TELEGRAM_TOKEN="<telegram-bot-token>"
export COPYLINGO_LLM_API_KEY="<gemini-api-key>"

make infra
make migrate
go run ./cmd/kana_seeder
go run ./cmd/server
```

기본적으로 Go 서버는 `:8080`에서 실행됩니다.

### 2. Cloudflare Tunnel로 HTTPS 공개 URL 발급

가장 단순한 개발 방식은 Cloudflare Tunnel입니다.

```bash
cloudflared tunnel --url http://localhost:8080
```

실행 후 `https://xxxxx.trycloudflare.com` 같은 공개 HTTPS URL이 출력됩니다.

### 3. public base URL 설정

터널에서 받은 URL을 `COPYLINGO_SERVER_PUBLIC_BASE_URL`로 설정한 뒤 서버를 다시 실행합니다.

```bash
export COPYLINGO_SERVER_PUBLIC_BASE_URL="https://xxxxx.trycloudflare.com"
go run ./cmd/server
```

또는 `config.yaml`에 직접 넣어도 됩니다.

```yaml
server:
  port: 8080
  mode: debug
  public_base_url: "https://xxxxx.trycloudflare.com"
```

### 4. BotFather에서 Mini App 도메인 설정

Telegram Mini App URL은 BotFather에 등록된 도메인과 일치해야 합니다.

필수 확인:
- BotFather에서 해당 봇의 Mini App/Web App 도메인 설정
- `public_base_url`의 host가 등록된 도메인과 일치하는지 확인
- tunnel URL이 바뀌면 `public_base_url`도 함께 갱신

### 5. 동작 확인 순서

1. Telegram에서 `/test`로 세션 생성
2. 손글씨 문항이 나오면 `✍️ 손글씨로 답하기` 버튼 클릭
3. Mini App이 열리면 canvas에 글자 입력 후 제출
4. Mini App 내부에서 채점 결과 확인
5. Telegram 채팅으로 돌아와 `제출 후 다음 문제 →` 버튼 클릭

### 6. 다른 머신에서 옮길 때 바꿔야 하는 것

- `COPYLINGO_TELEGRAM_TOKEN`
- `COPYLINGO_LLM_API_KEY`
- `COPYLINGO_SERVER_PUBLIC_BASE_URL`
- PostgreSQL / Redis 접근 정보
- Cloudflare Tunnel URL 또는 실제 운영 도메인

### 7. 주의사항

- `localhost`는 휴대폰 Telegram 앱에서 당신 PC를 가리키지 않습니다.
- Mini App은 HTTPS 공개 URL이 필요합니다.
- tunnel URL이 바뀌면 Bot이 생성하는 Mini App 링크도 즉시 바뀌므로, 서버 재시작 전 `public_base_url`을 맞춰야 합니다.
- `COPYLINGO_SERVER_PUBLIC_BASE_URL`이 비어 있으면 손글씨 문항에서 Mini App 버튼 대신 설정 경고가 출력됩니다.

---

## AI 설정 (Gemini 무료 티어)

[Google AI Studio](https://aistudio.google.com)에서 API 키 발급 후 `config.yaml` 또는 환경변수 설정:

```yaml
llm:
  api_key: "AIza..."
  model: "gemini-3.1-flash"
  base_url: "https://generativelanguage.googleapis.com/v1beta/openai/"
```

환경변수 예시:

```bash
export COPYLINGO_LLM_API_KEY="<gemini-api-key>"
export COPYLINGO_LLM_MODEL="gemini-3.1-flash"
```

| Gemini 3.1 Flash 무료 한도 | 예상 사용량 |
|---|---|
| 1,500 RPD | ~30회/일 (문제 생성 배치) |
| 15 RPM | 새벽 3시 배치, 무관 |

---

## 배포

```bash
# .env 파일 생성
echo "COPYLINGO_TELEGRAM_TOKEN=<토큰>" >> .env
echo "COPYLINGO_LLM_API_KEY=<Gemini API 키>" >> .env
echo "COPYLINGO_SERVER_PUBLIC_BASE_URL=https://copylingo.example.com" >> .env

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
