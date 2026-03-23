# CopyLingo — Agent System Prompt

> 이 문서는 AI 에이전트가 CopyLingo 프로젝트 작업 시 반드시 참고해야 하는 컨텍스트와 규칙을 정의합니다.

---

## 🚨 작업 시작 프로토콜 (필수)

새 대화에서 작업을 시작할 때 **반드시 아래 순서를 따르세요**:

```
1. AGENTS.md 읽기         ← 프로젝트 컨텍스트, 코딩 규칙 (첫 세션 또는 규칙 확인 시)
2. STATUS.md 읽기         ← 현재 작업 + 다음 작업 + 블로커
3. 작업 수행
4. STATUS.md 업데이트     ← 진행 중 → 완료, 다음 작업 설정
5. docs/workthrough/ 생성 ← YYMMDDhhmm_<job>.md 형식으로 상세 기록
```

> [!IMPORTANT]
> - `STATUS.md`의 "🔨 진행 중" 섹션이 에이전트의 작업 지시서입니다.
> - 작업 완료 시 반드시 `STATUS.md`를 업데이트하여 다음 에이전트가 이어갈 수 있게 하세요.
> - 작업 중 새 의사결정이 있으면 `docs/ADR.md`에 ADR 항목을 추가하세요.
> - 마일스톤 완료 시에만 `ROADMAP.md` 상태를 업데이트하세요.

---

## 프로젝트 개요

- **이름**: CopyLingo
- **목적**: 외국어 학습
- **핵심 플로우**: 콘텐츠 수집(뉴스/시험대비) → AI 문제 생성 → 텔레그램 푸시 → 풀이 → 채점 → SRS 복습
- **사용자**: 1명 (개인 사용)

## 기술 스택

| 구분 | 기술 | 비고 |
|---|---|---|
| 언어 | **Go 1.25** | |
| HTTP 프레임워크 | **Gin** | 헬스체크/관리 API 용도 |
| 텔레그램 | **go-telegram-bot-api/v5** | Inline Keyboard 기반 인터랙션 |
| DB | **PostgreSQL 16** | sqlx (raw SQL, ORM 미사용) |
| 캐시 | **Redis 7** | 세션 캐시, 응답 시간 측정 |
| 설정 | **Viper** | YAML + 환경변수 오버라이드 |
| 스케줄러 | **robfig/cron/v3** | |
| AI | **Gemini 3.0 Flash** | OpenAI 호환 엔드포인트 사용 |
| TTS | **Google Cloud TTS** | 사전 생성 + 파일 캐싱 |
| 컨테이너 | **Docker + Docker Compose** | PostgreSQL, Redis, App |

## 프로젝트 구조

```
copylingo/
├── cmd/server/main.go              ← 엔트리포인트 (DI, graceful shutdown)
├── internal/
│   ├── config/                      ← Viper 기반 설정
│   ├── model/                       ← 도메인 모델 (DB 매핑)
│   ├── repository/                  ← 데이터 접근 (PostgreSQL, raw SQL)
│   ├── service/                     ← 비즈니스 로직
│   ├── bot/                         ← 텔레그램 봇 핸들러
│   ├── scheduler/                   ← 크론 스케줄러
│   ├── pipeline/                    ← 콘텐츠 수집/문제 생성/TTS
│   └── external/                    ← 외부 API 클라이언트
├── migrations/                      ← SQL 마이그레이션
├── data/
│   ├── curriculum/                  ← N5~N1 커리큘럼 JSON
│   └── audio/                       ← TTS 캐시
├── docs/                            ← 프로젝트 문서
│   ├── workthrough/                 ← 작업 완료 기록 (YYMMDDhhmm_*.md)
│   ├── ARCHITECTURE.md              ← 시스템 아키텍처
│   └── ADR.md                       ← Architecture Decision Records
├── config.yaml                      ← 기본 설정 (민감 정보 제외)
├── docker-compose.yml
├── Dockerfile
├── Makefile
├── ROADMAP.md                       ← 전체 Phase/Subphase 진행 상황
└── STATUS.md                        ← 현재 작업 상태 (🚨 작업 전 필독)
```

## 코딩 규칙

### Go 코드

1. **패키지 구조**: `internal/` 하위에 레이어별 분리 (`model`, `repository`, `service`, `bot`, `pipeline`, `external`)
2. **DB 접근**: `sqlx`로 raw SQL 작성. ORM 사용 금지.
3. **에러 처리**: `fmt.Errorf("context: %w", err)` 패턴으로 에러 래핑
4. **ID**: DB PK는 SERIAL (auto-increment). users 테이블만 Telegram ID (BIGINT)
5. **Context**: 모든 repository/service 메서드는 첫 번째 인자로 `context.Context` 받기
6. **로깅**: 현재 `log` 표준 라이브러리 사용 (추후 structured logging 전환 가능)
7. **테스트**: `*_test.go` 파일, 같은 패키지 내 위치

### 텔레그램 봇

1. **Callback Data 규약**:
   - `session:{session_id}:start` — 세션 시작
   - `q:{session_id}:{question_id}:{option_idx}` — 답변 선택
   - `q:{session_id}:next:{current_idx}` — 다음 문제
   - `session:{session_id}:finish` — 결과 보기
   - `menu:{action}` — 메뉴 동작 (main, study, review, stats, settings)
2. **메시지 포맷**: HTML 파싱 모드 (`ParseMode = "HTML"`)
3. **키보드**: Inline Keyboard 사용 (Reply Keyboard 아님)

### DB

1. **마이그레이션**: `migrations/` 디렉토리에 `NNN_name.up.sql` / `NNN_name.down.sql` 쌍
2. **네이밍**: snake_case, 테이블명 복수형 (`users`, `questions`, `sessions`)
3. **Timestamp**: 모든 테이블에 `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
4. **JSONB**: 유연한 구조가 필요한 곳에 사용 (questions.options, sessions.questions, article_responses.conversation)
5. **인덱스**: 필요할 때만 추가. low-cardinality 컬럼(boolean, enum 등) 단독 인덱스 금지

### 설정

1. **민감 정보**: 환경변수로 주입 (`COPYLINGO_TELEGRAM_TOKEN`, `COPYLINGO_OPENAI_API_KEY`)
2. **기본값**: `config.yaml`에 비민감 기본값 정의
3. **환경변수 prefix**: `COPYLINGO_` (예: `COPYLINGO_DB_HOST`)
4. **AI 엔드포인트**: Gemini를 OpenAI 호환 모드로 사용
   ```yaml
   openai:
     api_key: "AIza..."
     model: "gemini-2.0-flash"
     base_url: "https://generativelanguage.googleapis.com/v1beta/openai/"
   ```

## 핵심 비즈니스 로직

### SM-2 간격 반복 (SRS)

- 위치: `internal/service/srs.go`
- 정답 시: interval 점진 증가 (1일 → 6일 → ×ease_factor)
- 오답 시: interval 리셋 → 1일
- ease_factor: 최소 1.3

### 세션 빌드 규칙

- 위치: `internal/service/session_builder.go`
- **오전 세션**: 15문제 = 새 문제 9개 (60%) + 복습 6개 (40%)
- **오후 세션**: 10문제 = 새 문제 6개 (60%) + 복습 4개 (40%)
- **콘텐츠 비율**: 뉴스 40% + 시험 대비 60%

### XP 계산 (채점)

- 위치: `internal/service/grader.go`
- 기본: 문제당 1 XP + 정답당 0.5 XP 보너스
- 퍼펙트 보너스: 전문 정답 시 +5 XP

## 의사결정 원칙

1. **개인 사용 최적화**: 소셜 기능, 과금 유도 장치(하트 등) 불필요
2. **Push 기반 학습**: 사용자가 찾아오는 게 아니라, 봇이 먼저 세션을 보냄
3. **AI 적극 활용**: 문제 생성, 아티클 대화, 피드백 모두 AI 기반
4. **AI**: Gemini 3.0 Flash (월 무료 1,500 RPD 내 운용)

## 작업 시 주의사항

> [!CAUTION]
> `config.yaml`에 API 키나 토큰을 절대 하드코딩하지 마세요. 환경변수로 주입합니다.

## 개발 명령어

```bash
make infra      # PostgreSQL + Redis 시작
make migrate    # DB 마이그레이션 실행
make run        # 앱 실행 (go run)
make build      # 바이너리 빌드
make test       # 전체 테스트
make docker-up  # Docker 전체 시작 (앱 포함)
```

## 참고 문서

- [STATUS.md](STATUS.md) — 현재 작업 상태 (🚨 작업 전 필독)
- [ROADMAP.md](ROADMAP.md) — 전체 Phase/Subphase 진행 상황
- [아키텍처](docs/ARCHITECTURE.md) — 시스템 구조, 데이터 흐름, 콜백 규약
- [ADR](docs/ADR.md) — 기술 의사결정 기록
- [작업 기록](docs/workthrough/) — 완료된 작업 상세 기록
