# CopyLingo — Agent Contract (SSOT)

> AI agent가 CopyLingo 작업 시 따라야 할 공통 규칙입니다. **Codex는 이 파일을 직접 자동 로드**하고, **Claude / Gemini는 각자의 파일(`CLAUDE.md`, `GEMINI.md`)에서 이 문서를 가리킵니다.** 즉 이 문서는 모든 agent의 SSOT입니다.

---

## 1. agent 진입 규칙

각 CLI는 자기 컨벤션 파일을 자동 로드합니다. 이 매핑은 도구 측 동작이라 변경할 수 없습니다.

| agent | 자동 로드 파일 | AGENTS.md 접근 방법 |
|---|---|---|
| Claude Code | `CLAUDE.md` | `CLAUDE.md` 첫 줄에서 본 문서 위임 |
| Codex | `AGENTS.md` | 본 문서를 직접 로드 |
| Gemini CLI | `GEMINI.md` | `GEMINI.md` 첫 줄에서 본 문서 위임 |

각 agent별 파일은 **이 문서 위에 얹는 얇은 overlay**입니다. 공통 규칙은 모두 이 문서에 있습니다.

---

## 2. agent 역할 matrix

| 주체 | 진입 방식 | 책임 범위 |
|---|---|---|
| **사용자** | 모든 session의 출발점 | **최종 결정권자.** 모든 의사결정(설계, 구현 방향, ADR 채택, TODO 위임 여부 등)에 대한 최종 승인. agent는 제안·실행·review까지 하지만, **비자명한 결정은 반드시 사용자 confirmation을 받음** |
| **Claude Code / Codex** | 사용자가 직접 session 시작 | 한 작업을 **설계 · 구현 · 검증 · review**까지 끝까지 진행. 두 agent는 **사용자 선호로 선택** (의미 있는 능력 차이가 생기면 본 문서에 명시). 비자명한 판단은 사용자에게 올림 |
| **Gemini CLI** | main session에서 만든 TODO 문서를 받음 | **자기완결 명세를 명세 그대로 실행**. 판단이 필요한 지점이 나오면 즉시 중단하고 사용자에게 질문 |

> Gemini CLI는 **main 작업 흐름에서 떨어져 나온 자기완결 TODO 실행자**입니다. 자기완결적이지 않은 작업은 보내지 않습니다.

### 역할 대체 / fallback

> 역할 matrix는 기본값이며 권한 제한이 아닙니다. 사용자가 직접 다른 배정을 지정하면 해당 agent가 수행하되, §3 작업 protocol은 동일하게 적용합니다.

---

## 3. 작업 protocol — 3 cases

사용자의 요청은 다음 셋 중 하나로 분류되며, 분류에 따라 산출물과 절차가 다릅니다.

> **사전 분기 — 극소 변경: 사용자가 직접 처리**

>  **매우 작은 변경**(e.g. 글자 하나, 변수명 하나, 단일 오타)처럼 agent의 토큰 소모가 비효율적인 요청을 받으면 처리하지 말고 **"이건 직접 수정하는 게 토큰 효율적입니다"** 라고 안내하세요. 사용자가 "그래도 진행해"라고 지시하면 그때 처리.

### Case A. 의사결정 (ADR)

> 시스템 아키텍처, 구현 방식, tradeoff 등 **코드 베이스 구성에 관한 결정**을 논의하는 경우.

- **담당**: Claude / Codex
- **절차**:
  1. 사용자와 충분히 논의. 가정한 규모(§4 참조)에서의 tradeoff 명시
  2. **결정이 굳어지면 사용자가 별도로 요청하지 않아도 agent가 즉시 `docs/ADR.md`에 항목 추가** (배경 / 결정 / 결과). 사용자가 ADR 갱신을 잊는 일이 잦으므로 agent가 능동적으로 처리할 것
  3. 코드 변경이 동반되면 Case B로 이어서 진행
- **주의**: "현재 1인 사용이라 괜찮다", "YAGNI" 같은 답변을 default로 깔지 말 것 (§4 참조)

### Case B. 코드 작성

> 작업 범위가 정해진 후 **plan 작성 → 구현 → 검증 → 종료** 까지 한 흐름에서 처리하는 경우.

- **담당**: Claude / Codex (사용자 선택)
- **절차**:
  1. **시작**: `STATUS.md`에서 "🔨 진행 중" 항목 확인 — 현재 요청과 관련된지 판단
  2. **plan**: 비자명한 작업이면 plan을 사용자와 합의한 뒤 구현 시작
  3. **구현**: `internal/` 레이어 구조와 코딩 규칙(§5) 준수
  4. **검증**: 코드/마이그레이션/설정 변경 시 `make test` **필수**. 문서-only 작업이면 미실행 사유를 workthrough에 기록
  5. **종료**:
     - `STATUS.md` 갱신 — **현재 요청이 "진행 중" 항목 자체를 완료하는 경우에만** "진행 중" → "📝 최근 완료"로 이동. 진행 중 항목과 무관한 side task(예: 문서 정리, 우발적 발견 처리)는 STATUS.md를 건드리지 않거나 "📝 최근 완료"에 한 줄만 추가
     - trivial하지 않은 작업에 대해 `docs/workthrough/YYMMDDhhmm_<job>.md` 생성 — 변경 파일, 결정 사항, 검증 결과
     - 의사결정이 발생했다면 `docs/ADR.md` 갱신
     - 마일스톤 완료 시에만 `ROADMAP.md` 갱신
- **언어**: implementation plan과 workthrough는 **한글**로 작성

### Case C. TODO 분리 및 위임

> 작업 중 **현재 범위 밖의 이상치/개선 포인트를 발견**했고, 즉시 처리하지 않고 기록만 남기는 경우.

- **트리거**: 사용자 혹은 agent가 코드 review/구현 중 현재 scope 밖의 이슈를 발견했지만, 즉시 처리하면 작업 범위가 과도하게 커진다고 판단해 TODO로 남기기로 결정
- **담당**:
  - **분리 결정**: 사용자
  - **문서 작성**: 현재 main session의 agent
  - **실행**: 별도 session의 담당 agent

#### 분리 (main agent)

1. **`docs/todos/<task>.md`를 자기완결적 plan 문서로 반드시 작성**한다 (한 줄 요약만 남기는 escape hatch는 없음). 다음 항목을 포함:
   - 배경/목적
   - 변경할 파일 목록 + Before/After 스니펫
   - 검증 방법 (`make test` 등)
   - 건드리면 안 되는 영역, 결정된 사항
2. plan 작성 중 모호한 결정 포인트가 있으면 **이 단계에서 사용자와 합의해 문서에 박아둘 것** (실행 agent가 다시 묻지 않도록)
3. `STATUS.md`에 한 줄 요약 + 문서 링크 등록:
   `- [ ] <한 줄 요약> — see [docs/todos/<file>.md](docs/todos/<file>.md)`

#### 실행 (담당 agent, 주로 Gemini)

1. `STATUS.md`에서 plan 문서 경로 확인 → `docs/todos/<task>.md`를 **처음부터 끝까지 정독**
2. 분기:
   - 명확하지 않은 사항이 있으면 → **사용자에게 질문** (plan에 박힌 결정 외 추가 판단이 필요한 경우)
   - 명확하면 → **즉시 작업 시작**. 이후는 **Case B의 절차 3(구현) → 4(검증) → 5(종료)를 그대로 따른다.**
3. **Case C 종료 시 추가 처리** (Case B 종료에 더해서):
   - `STATUS.md`에서 해당 TODO 체크박스 항목 제거 (Case B의 "진행 중 → 최근 완료 이동" 규칙과는 별개)
   - `docs/todos/<task>.md` 삭제 (git history에 보존됨)

### Case 간 전이

작업 도중 case가 바뀌는 경우는 흔하다. 다음 전이는 모두 명시적으로 처리한다 (암묵적 case 변경 금지):

| 전이 | 트리거 | 처리 |
|---|---|---|
| **A → B** | ADR 결정 후 코드 변경이 동반됨 | ADR 항목 기록 후 곧바로 Case B 절차로 진입 |
| **B → A** | 구현 중 가정한 규모/아키텍처에 영향을 주는 **비자명한 결정**이 새로 등장 | **구현을 일시 중단** → 사용자와 Case A 절차로 합의 → ADR 즉시 기록 → Case B 복귀 |
| **B → C** | 작업 범위 밖의 이상치/개선 포인트 발견 | 현재 구현은 멈추지 않고 Case C로 TODO 분리만 수행 → 분리 후 원래 Case B 계속 |
| **C → B** | 실행 단계에서 plan에 명시되지 않은 비자명한 결정이 필요하거나, 작업 범위가 plan보다 훨씬 큼이 드러남 | 실행 agent는 즉시 중단·사용자에게 보고 → 사용자 판단으로 plan 보강 후 재개하거나, main session의 Case B로 격상해 정상 처리 |

---

## 4. 프로젝트 성격 및 설계 기준 ⚠️

> 본 프로젝트의 **모든 아키텍처/리팩터 결정은 이 섹션을 기준**으로 평가됩니다.

- CopyLingo는 **(a) 실제 외국어 학습용 + (b) 포트폴리오**의 dual-purpose 프로젝트이며, **coding 시 우선순위는 (b)** 입니다.
- 실제 사용자는 1명이지만, 아키텍처/리팩터 결정은 **"이 시스템이 수만~수십만 사용자를 다룬다"는 가정** 하에 평가합니다.
- 이유: 더 큰 규모 시스템을 다뤄본 경험을 쌓는 것이 이 프로젝트의 핵심 목적 중 하나입니다.

### 적용 방법

- **"현재 1인 부하라 괜찮다", "YAGNI" 같은 답변을 default로 깔지 말 것.** 그 결론을 사용자가 명시적으로 원할 때만 그쪽으로 갑니다.
- 가정한 규모에서 실제 의미가 있는 패턴(캐시 SSOT, 이벤트 스트림/outbox, CQRS, 비동기 워커 등)을 의식적으로 고르고 tradeoff를 명시.
- 단, "스케일을 가정한다"는 것이 모든 곳에 분산 시스템을 박는다는 뜻은 아님. **의도된 선택**이어야 함.
- 코드만큼 **`docs/ADR.md` 등 설계 문서가 1급 산출물**. 비자명한 설계 변경 시 ADR 갱신을 함께 수행.

### 의사결정 원칙

1. **개인 사용 최적화는 기능 scope 결정에만 적용** — 소셜 기능, 과금 유도 장치(하트 등) 불필요. **아키텍처/성능 결정에는 적용 금지.**
2. **Push 기반 학습** — 사용자가 찾아오는 게 아니라, 봇이 먼저 session을 보냄
3. **AI 적극 활용** — 문제 생성, 아티클 대화, 피드백 모두 AI 기반
4. **AI 모델 운용** — Gemini를 OpenAI 호환 모드로 사용 (월 무료 1,500 RPD 내 운용)

---

## 5. 코딩 규칙

### Go 코드

1. **패키지 구조**: `internal/` 하위에 레이어별 분리 (`model`, `repository`, `service`, `bot`, `pipeline`, `external`)
2. **DB 접근**: `sqlx`로 raw SQL 작성. 
3. **에러 처리**:
   - 에러 발생 지점에서는 로그를 찍지 말고 `fmt.Errorf("context: %w", err)` 패턴으로 맥락을 붙여 반환
   - Repository 계층은 함수명/주요 식별자 기반으로 검색 가능한 에러 컨텍스트 포함 (예: `SessionQuestionRepository.GetBySession session_id=%d: %w`)
   - Service 계층은 새로운 비즈니스 의미를 추가할 때만 래핑. 단순 repository pass-through 함수는 그대로 반환
   - `err`를 이후에 재사용하지 않으면 `if err := ...; err != nil` 또는 `if _, err := ...; err != nil` 형태로 스코프를 좁히기
4. **ID**: DB PK는 SERIAL (auto-increment). users 테이블만 Telegram ID (BIGINT)
5. **Context**: 모든 repository/service 메서드는 첫 번째 인자로 `context.Context` 받기
6. **로깅**:
   - 현재 `log` 표준 라이브러리 사용 (추후 structured logging 전환 가능)
   - Repository 같은 하위 계층에서는 직접 로그를 찍지 않음
   - Bot handler, HTTP handler, scheduler job 같은 경계 계층에서 사용자/작업 맥락과 함께 한 번만 로그 출력
7. **테스트**: `*_test.go` 파일, 같은 패키지 내 위치

### 텔레그램 봇

1. **Callback Data 규약**:
   - `session:{session_id}:start` — session 시작
   - `q:{session_id}:{question_id}:{option_idx}` — 답변 선택
   - `q:{session_id}:next:{current_idx}` — 다음 문제
   - `session:{session_id}:finish` — 결과 보기
   - `menu:{action}` — 메뉴 동작 (main, study, review, stats, settings)
2. **메시지 포맷**: HTML 파싱 모드 (`ParseMode = "HTML"`)
3. **키보드**: Inline Keyboard 사용 (Reply Keyboard 아님)

### DB

1. **마이그레이션**: `migrations/` 디렉토리에 `NNN_name.sql` **단일 파일**. `make migrate` 가 `NNN_*.sql` 을 파일명 순으로 일괄 적용.
2. **네이밍**: snake_case, 테이블명 복수형 (`users`, `questions`, `sessions`)
3. **Timestamp**: 모든 테이블에 `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
4. **JSONB**: 유연한 구조가 필요한 곳에 사용 (questions.options, sessions.questions, article_responses.conversation)
5. **인덱스**: 필요할 때만 추가. low-cardinality 컬럼(boolean, enum 등) 단독 인덱스 금지

### 설정

- **민감 정보는 환경변수로 주입** (`COPYLINGO_TELEGRAM_TOKEN`, `COPYLINGO_LLM_API_KEY` 등). `config.yaml`에 API 키/토큰 하드코딩 금지.

---

## 6. 참고 문서

- [README.md](README.md) — 프로젝트 개요, 기술 스택, 로컬 개발/배포 방법
- [STATUS.md](STATUS.md) — 현재 작업 상태 (🚨 작업 전 필독)
- [ROADMAP.md](ROADMAP.md) — 전체 Phase/Subphase 진행 상황
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) — 시스템 구조, 데이터 흐름, 콜백 규약
- [docs/ADR.md](docs/ADR.md) — 기술 의사결정 기록
- [docs/workthrough/](docs/workthrough/) — 완료된 작업 상세 기록
- [docs/todos/](docs/todos/) — 별도 session에서 실행될 TODO의 자기완결 plan 문서
- [Makefile](Makefile) — 개발 명령어 (`make test`, `make infra`, `make migrate`, `make build` 등). README.md "Makefile" 섹션에도 표로 정리됨
