# 2026-05-20 Uncommitted 변경 리뷰 플랜

## 목적

현재 worktree 에 섞여 있는 uncommitted 변경을 시간순 task 단위로 분리해 리뷰한다. 각 대화 탭은 아래 섹션 하나만 맡아도 독립적으로 리뷰/수정할 수 있어야 한다.

## 현재 검증 상태

- `make test` 통과
- 리뷰 기준 문서:
  - [2605110132_agent_docs_restructure.md](workthrough/2605/2605110132_agent_docs_restructure.md)
  - [2605200103_handwriting_tips_integration.md](workthrough/2605/2605200103_handwriting_tips_integration.md)

## 전체 리뷰 순서

1. DB / migration / schema 변경
2. Tips domain / repository / service
3. Mini App Tips API / frontend 연동
4. Telegram post-submit button cleanup
5. 문서 / STATUS / TODO 정합성
6. `.claude/settings.json` 포함 여부 결정

---

## 탭 4. Mini App Tips API / Frontend

### 범위

리뷰 파일 순서:
1. [internal/config/constants.go](../internal/config/constants.go)
2. [internal/miniapp/handler.go](../internal/miniapp/handler.go)
3. [internal/miniapp/handler_test.go](../internal/miniapp/handler_test.go)
4. [web/miniapp/handwriting/index.html](../web/miniapp/handwriting/index.html)
5. [web/miniapp/handwriting/app.js](../web/miniapp/handwriting/app.js)
6. [web/miniapp/handwriting/style.css](../web/miniapp/handwriting/style.css)

### 리뷰 포인트

- `GET /api/miniapp/tips` 가 public read-only endpoint 로 열리는 것이 ADR-015 와 맞는지 확인.
- empty result 가 `[]` 로 반환되는지 확인.
- repository error 시 `500` 반환 테스트가 충분한지 확인.
- `limit` parsing 정책: invalid/negative 값은 default 30 으로 무시한다. 이 정책을 허용할지 확인.
- frontend `loadTips()` 실패 시 spinner-only graceful degradation 이 맞는지 확인.
- `TIP_CATEGORY_DISPLAY` 가 Go `TipCategory.DisplayName()` 과 중복된다. 서버 응답에 display label 을 추가하지 않은 결정을 유지할지 확인.
- asset cache busting query `v=2605200114` 를 수동 상수로 둘지 확인.
- 같은 WebView session 안에서 `(language, level)` 기준 `sessionStorage` cache 로 반복 tips API 호출을 줄인다.

### 예상 리뷰 이슈 후보

- `app.js` 의 `shuffle(tips)` 는 서버가 배열을 보장한다는 전제다. 추가로 `Array.isArray()` guard 를 둘지 결정 가능. → 반영 완료.
- CSS `border-radius: 16px` 는 현재 frontend guidance 의 “cards 8px 이하” 선호와 어긋날 수 있다. 기존 miniapp 디자인 우선인지 확인.

---

## 탭 5. Telegram Post-submit Button Cleanup

### 범위

참조:
- [2605200103_handwriting_tips_integration.md](workthrough/2605/2605200103_handwriting_tips_integration.md) 의 “부수 변경” 섹션

리뷰 파일 순서:
1. [internal/callback/callback.go](../internal/callback/callback.go)
2. [internal/bot/handler.go](../internal/bot/handler.go)
3. [internal/bot/session_flow.go](../internal/bot/session_flow.go)
4. [internal/bot/session_flow_test.go](../internal/bot/session_flow_test.go)
5. [cmd/server/main.go](../cmd/server/main.go)
6. [cmd/server/server.go](../cmd/server/server.go)
7. [internal/miniapp/handler.go](../internal/miniapp/handler.go)
8. [internal/miniapp/handler_test.go](../internal/miniapp/handler_test.go)

### 리뷰 포인트

- [internal/callback](../internal/callback/callback.go) 패키지 분리가 callback data SSOT 로 적절한지 확인.
- `SendMessageWithReplyMarkup` 시그니처 변경의 호출자 영향이 모두 반영됐는지 확인.
- Redis key `handwriting:msg:{session_id}:{question_id}` TTL 1시간이 적절한지 확인.
- Mini App submit 성공 후 `EditMessageReplyMarkup` 를 goroutine 으로 best-effort 처리하는 정책이 맞는지 확인.
- `parseHandwritingMessageRef` validation 이 충분한지 확인.
- `refreshHandwritingMessage` 가 `context.Background()` 를 쓰는 것이 request cancellation 과 독립 처리 목적에 맞는지 확인.
- `SessionBuilder.GetSessionQuestions` 를 cleanup path 에서 추가 호출하는 비용을 허용할지 확인.

### 예상 리뷰 이슈 후보

- `refreshHandwritingMessage` 성공 경로 자체는 integration test 가 없다. fake Redis/fake messenger 기반 테스트를 추가할지 결정 가능.
- message cleanup 은 flow-local UX cleanup 으로 ADR-016 을 제거했다. 이 판단을 유지할지 확인.

---

## 탭 6. 문서 / STATUS / TODO 정합성

### 범위

리뷰 파일 순서:
1. [docs/ADR.md](ADR.md)
2. [STATUS.md](../STATUS.md)
3. [docs/todos/tip_scheduler_generation.md](todos/tip_scheduler_generation.md)
4. [2605200103_handwriting_tips_integration.md](workthrough/2605/2605200103_handwriting_tips_integration.md)

### 리뷰 포인트

- ADR-015 가 현재 구현 상태를 정확히 반영하는지 확인.
- [tip_scheduler_generation.md](todos/tip_scheduler_generation.md) 가 실행 agent 에게 자기완결적인지 확인.
- 완료된 `handwriting_post_submit_button_cleanup` TODO 가 STATUS 에서 제거된 상태가 맞는지 확인.
- workthrough 의 검증 내역이 실제 실행 결과와 맞는지 확인.

### 결정 필요

- [tip_scheduler_generation.md](todos/tip_scheduler_generation.md) 를 이번 commit 에 같이 포함할지, 별도 commit 으로 분리할지 결정.

---

## 권장 commit 분리

### Commit 1. Agent 문서 정리

포함 후보:
- [AGENTS.md](../AGENTS.md)
- [.claude/settings.json](../.claude/settings.json) 은 포함 여부 별도 결정

### Commit 2. Migration 컨벤션 + tips schema

포함 후보:
- [Makefile](../Makefile)
- [migrations/001_init.sql](../migrations/001_init.sql)
- `migrations/001_init.up.sql` (삭제 파일)
- `migrations/001_init.down.sql` (삭제 파일)
- [schema.dbml](../schema.dbml)
- [docs/ADR.md](ADR.md) 중 ADR-015

### Commit 3. Tips API + Mini App UX

포함 후보:
- [internal/model/tip.go](../internal/model/tip.go)
- [internal/repository/tip_repo.go](../internal/repository/tip_repo.go)
- [internal/repository/repositories.go](../internal/repository/repositories.go)
- [internal/service/tip.go](../internal/service/tip.go)
- [internal/service/services.go](../internal/service/services.go)
- [internal/config/constants.go](../internal/config/constants.go)
- [internal/miniapp/handler.go](../internal/miniapp/handler.go)
- [internal/miniapp/handler_test.go](../internal/miniapp/handler_test.go)
- [web/miniapp/handwriting/](../web/miniapp/handwriting/)

### Commit 4. Telegram post-submit button cleanup

포함 후보:
- [internal/callback/callback.go](../internal/callback/callback.go)
- [internal/bot/handler.go](../internal/bot/handler.go)
- [internal/bot/session_flow.go](../internal/bot/session_flow.go)
- [internal/bot/session_flow_test.go](../internal/bot/session_flow_test.go)
- [cmd/server/main.go](../cmd/server/main.go)
- [cmd/server/server.go](../cmd/server/server.go)
- [internal/config/constants.go](../internal/config/constants.go)
- [internal/miniapp/handler.go](../internal/miniapp/handler.go)
- [internal/miniapp/handler_test.go](../internal/miniapp/handler_test.go)

주의: [internal/config/constants.go](../internal/config/constants.go), [internal/miniapp/handler.go](../internal/miniapp/handler.go), [internal/miniapp/handler_test.go](../internal/miniapp/handler_test.go) 는 Commit 3/4 에 걸쳐 있어 실제 commit 분리 시 `git add -p` 가 필요하다.

### Commit 5. 작업 상태 문서

포함 후보:
- [STATUS.md](../STATUS.md)
- [docs/todos/tip_scheduler_generation.md](todos/tip_scheduler_generation.md)
- [2605200103_handwriting_tips_integration.md](workthrough/2605/2605200103_handwriting_tips_integration.md)
- 본 문서 [docs/260520_uncommitted_review_plan.md](260520_uncommitted_review_plan.md)

---

## 리뷰 반영 운영 방식

1. 탭 하나를 선택한다.
2. 해당 탭의 “리뷰 파일 순서”대로 확인한다.
3. 리뷰 코멘트는 해당 탭 섹션 아래에 bullet 로 추가하거나, 바로 코드 수정한다.
4. 코드 수정이 있으면 최소 `go test ./...`, 최종 merge 전 `make test` 를 실행한다.
5. task 범위가 커지면 `docs/todos/<task>.md` 로 분리한다.
