# CopyLingo Onboarding Review Flow

## Scope

- 신규 팀원이 CopyLingo의 런타임 흐름을 이해하기 위한 1차 코드 읽기 순서입니다.
- 범위는 서버 부팅, DB/Redis 상태 모델, Quiz Session, Study Session, Scheduler, Mini App, AI grading의 주요 경계입니다.

## Route Summary

| Category | Purpose |
|---|---|
| Project Map | 프로젝트 목적, 현재 진행 상태, 운영 명령어를 먼저 파악한다 |
| Trigger & Config | 서버가 어떤 설정으로 어떤 dependency를 붙이는지 확인한다 |
| Schema & Models | PostgreSQL 테이블과 Go model의 관계를 잡는다 |
| Dependency Wiring | repository/service/bot/scheduler가 조립되는 지점을 본다 |
| Quiz Session Flow | 문제 풀이 세션이 생성, 진행, 채점, 완료되는 경로를 따라간다 |
| Study Session Flow | Material 기반 정오 Study Session의 Redis working set과 DB flush를 따라간다 |
| Boundary Flow | Telegram callback, Scheduler, Mini App HTTP endpoint를 연결해서 본다 |
| External & Pipeline | LLM grading과 콘텐츠 수집 파이프라인의 외부 경계를 본다 |
| Tests | 변경 전 가장 먼저 돌려볼 테스트 축을 확인한다 |

## Review Order

### 1. Project Map

1. [README.md:1](../../README.md) — 프로젝트 목적과 로컬 실행 흐름을 먼저 잡는다.
2. [STATUS.md:7](../../STATUS.md) — 현재 진행 중 작업이 `Phase 2.4: 아티클 요약 및 AI 대화 시나리오 구현`임을 확인한다.
3. [Makefile:1](../../Makefile) — `make test`, `make infra`, `make migrate`, `make restart-app`의 실제 동작을 확인한다.
4. [docs/ARCHITECTURE.md:29](../../docs/ARCHITECTURE.md) — 문서상 레이어 구조와 실제 `internal/` 구성을 대조한다.

### 2. Trigger & Config

1. [cmd/server/main.go:20](../../cmd/server/main.go) — `run()`에서 config, logging, infra, app, workers, HTTP server가 순서대로 뜬다.
2. [internal/config/config.go:13](../../internal/config/config.go) — 전체 설정 구조와 env override 이름을 확인한다.
3. [internal/config/constants.go:14](../../internal/config/constants.go) — Telegram callback prefix, Redis key, Mini App route의 SSOT를 확인한다.
4. [cmd/server/server.go:28](../../cmd/server/server.go) — PostgreSQL/Redis 초기화와 cleanup 책임을 본다.
5. [cmd/server/server.go:147](../../cmd/server/server.go) — Gin router가 health check와 Mini App routes만 등록하는 구조를 확인한다.

### 3. Schema & Models

1. [migrations/001_init.sql:7](../../migrations/001_init.sql) — `users`, `contents`, `materials`, `questions`, `sessions` 계열 테이블을 DB 관점에서 먼저 본다.
2. [internal/model/question.go:8](../../internal/model/question.go) — `QuestionType`, `Skill`, `QuestionCategory`가 문제 렌더링/분류의 기준이다.
3. [internal/model/session.go:5](../../internal/model/session.go) — `SessionType`, `SessionMode`, `SessionStatus`로 Quiz와 Study를 구분한다.
4. [internal/model/material.go:8](../../internal/model/material.go) — Study Session의 학습 단위인 `Material`과 user progress 모델을 확인한다.
5. [internal/model/active_session.go:5](../../internal/model/active_session.go) — Quiz 진행 중 Redis working set의 in-memory shape를 본다.
6. [internal/model/study_active_session.go:5](../../internal/model/study_active_session.go) — Study 진행 중 Redis working set의 in-memory shape를 본다.

### 4. Dependency Wiring

1. [internal/repository/repositories.go:9](../../internal/repository/repositories.go) — 모든 repository 생성 지점이며 DB dependency가 여기서 퍼진다.
2. [internal/service/services.go:11](../../internal/service/services.go) — service graph와 LLM client, Redis working set dependency가 조립된다.
3. [cmd/server/server.go:41](../../cmd/server/server.go) — `initApp`에서 repository, service, bot이 붙는다.
4. [cmd/server/server.go:51](../../cmd/server/server.go) — scheduler와 Telegram polling goroutine이 시작된다.
5. [internal/bot/handler.go:51](../../internal/bot/handler.go) — Telegram Bot wrapper가 `SessionFlow`와 `StudyFlow`를 소유한다.

### 5. Quiz Session Flow

1. [internal/service/session_builder.go:69](../../internal/service/session_builder.go) — Morning/Evening/Review Quiz Session 생성 정책을 본다.
2. [internal/repository/question_repo.go:44](../../internal/repository/question_repo.go) — 새 문제 조회와 due review 조회 조건을 확인한다.
3. [internal/repository/session_repo.go:21](../../internal/repository/session_repo.go) — `sessions` row 생성과 status 전이를 확인한다.
4. [internal/repository/session_question_repo.go:12](../../internal/repository/session_question_repo.go) — `session_questions` 생성/조회가 Quiz item 순서를 만든다.
5. [internal/service/active_session.go:47](../../internal/service/active_session.go) — 진행 중 Quiz 상태를 Redis working set으로 관리한다.
6. [internal/repository/active_session_repo.go:63](../../internal/repository/active_session_repo.go) — DB에서 full session state를 한 번에 load한다.
7. [internal/repository/active_session_repo.go:178](../../internal/repository/active_session_repo.go) — 완료 시 session/question/SRS 상태를 transaction으로 flush한다.
8. [internal/service/grader.go:45](../../internal/service/grader.go) — 답안 채점과 active session 기록 경로를 확인한다.

### 6. Study Session Flow

1. [internal/service/study_session.go:43](../../internal/service/study_session.go) — 정오 Study Session이 8개 Material을 골라 `sessions`와 `session_materials`를 만든다.
2. [internal/repository/material_repo.go:21](../../internal/repository/material_repo.go) — due/new vocabulary material selection SQL을 본다.
3. [internal/repository/session_material_repo.go:21](../../internal/repository/session_material_repo.go) — Study Session의 ordered material join row 생성 지점이다.
4. [internal/service/study_active_session.go:34](../../internal/service/study_active_session.go) — Study 진행 상태의 Redis working set 책임을 확인한다.
5. [internal/repository/study_active_session_repo.go:54](../../internal/repository/study_active_session_repo.go) — DB에서 Study Session과 Material snapshot을 load한다.
6. [internal/repository/study_active_session_repo.go:140](../../internal/repository/study_active_session_repo.go) — 완료 시 studied_at과 user_material_progress를 flush한다.
7. [internal/model/study_active_session.go:18](../../internal/model/study_active_session.go) — studied count, next unstudied, newly studied 계산의 기준을 확인한다.

### 7. Boundary Flow

1. [internal/bot/handler.go:74](../../internal/bot/handler.go) — Telegram polling loop가 update마다 goroutine으로 handler를 호출한다.
2. [internal/bot/handler.go:299](../../internal/bot/handler.go) — callback data prefix가 `SessionFlow`, `StudyFlow`로 라우팅된다.
3. [internal/bot/session_flow.go:153](../../internal/bot/session_flow.go) — Quiz Session start/finish callback 처리 진입점이다.
4. [internal/bot/session_question.go:18](../../internal/bot/session_question.go) — 문제 렌더링, current index 저장, Mini App URL 생성이 모이는 지점이다.
5. [internal/bot/session_answer.go:18](../../internal/bot/session_answer.go) — 객관식/텍스트 답변 처리와 feedback rendering 흐름을 본다.
6. [internal/bot/study_flow.go:38](../../internal/bot/study_flow.go) — Study callback start/next/finish가 active study service로 이어진다.
7. [internal/scheduler/scheduler.go:45](../../internal/scheduler/scheduler.go) — cron job 등록과 `buildAndPush*` 호출 흐름을 본다.
8. [internal/miniapp/handler.go:83](../../internal/miniapp/handler.go) — Mini App static route, tips API, handwriting submit endpoint가 등록된다.
9. [internal/miniapp/handler.go:140](../../internal/miniapp/handler.go) — Telegram init data 검증 후 handwriting service로 제출이 넘어간다.

### 8. External & Pipeline

1. [internal/service/handwriting.go:80](../../internal/service/handwriting.go) — Mini App stroke submit을 active session 검증, PNG render, grading으로 연결한다.
2. [internal/service/handwriting_render.go:15](../../internal/service/handwriting_render.go) — stroke JSON을 서버 PNG로 rebuild하는 렌더러를 확인한다.
3. [internal/external/llm.go:17](../../internal/external/llm.go) — subjective answer와 handwriting grading의 LLM boundary contract를 본다.
4. [internal/external/llm.go:117](../../internal/external/llm.go) — handwriting visual grading prompt와 strict JSON schema를 확인한다.
5. [internal/pipeline/orchestrator.go:16](../../internal/pipeline/orchestrator.go) — content collection pipeline의 fetch-process-save orchestration을 본다.
6. [cmd/server/server.go:92](../../cmd/server/server.go) — NHK pipeline wiring이 scheduler의 content collection job에 연결된다.

### 9. Tests

1. [internal/service/study_active_session_test.go:1](../../internal/service/study_active_session_test.go) — Study Redis working set과 completion edge case를 먼저 확인한다.
2. [internal/repository/study_active_session_repo_test.go:1](../../internal/repository/study_active_session_repo_test.go) — Study flush transaction과 progress update 검증을 본다.
3. [internal/service/active_session_test.go:1](../../internal/service/active_session_test.go) — Quiz Redis working set의 answer/flush behavior를 확인한다.
4. [internal/bot/session_flow_test.go:1](../../internal/bot/session_flow_test.go) — Telegram Quiz interaction flow의 사용자-facing behavior를 본다.
5. [internal/bot/study_flow_test.go:1](../../internal/bot/study_flow_test.go) — Telegram Study interaction flow의 callback behavior를 본다.
6. [internal/miniapp/handler_test.go:1](../../internal/miniapp/handler_test.go) — Mini App HTTP boundary의 auth/error mapping을 확인한다.
7. [internal/external/llm_test.go:1](../../internal/external/llm_test.go) — LLM response parsing과 error handling contract를 확인한다.
8. [internal/scheduler/scheduler_test.go:1](../../internal/scheduler/scheduler_test.go) — Scheduler job registration/build-push behavior를 확인한다.

## Notes

- README는 `docs/ADR.md`, `docs/HISTORY.md`, `CURRENT_TASK.md`를 언급하지만 현재 파일 목록에서는 확인되지 않았고, 실제 ADR은 `docs/adr/ADR_from_01_to_20.md`, `docs/adr/ADR_from_21_to_40.md` 형태로 보인다.
- Architecture 문서의 daily pipeline 설명은 “오전 build cron”을 말하지만, 현재 `internal/scheduler/scheduler.go`는 push cron에서 build와 push를 함께 수행한다.
- `SessionBuilderService.buildSession`에는 due review 조회가 language/level로 필터링되지 않는 TODO가 남아 있다.
- `QuestionRepository.CreateBatch`는 repository 계층에서 직접 `log.Println`을 호출한다. 프로젝트 규칙상 repository는 wrapping error만 반환하는 방향과 어긋난다.
- 신규 팀원에게는 Quiz Session과 Study Session을 한 번에 섞어 읽히지 말고, 먼저 Quiz Session을 끝까지 따라간 뒤 Study Session을 별도 경로로 읽히는 것을 권장한다.
