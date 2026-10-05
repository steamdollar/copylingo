# E2E Test 보강 계획 (for Gemini)

> 목표: **핵심 사용자 시나리오 전체 경로**(입력 → handler → service → repository → DB/Redis)가 끝까지 동작함을 보장. 시나리오 단위 regression 차단.
> **제약: 테스트 코드만 작성한다. production 코드는 절대 수정하지 않는다.**
> 현재 e2e는 0건. 적은 수(2~4개)로 큰 안전망을 만드는 게 목표 — 망라가 아니라 "핵심 동맥" 보호.

---

## 0. 범위 정의

이 프로젝트의 두 진입점:
1. **Telegram Bot** ([internal/bot](../../internal/bot)) — update 수신 → 세션 진행
2. **MiniApp HTTP** ([internal/miniapp](../../internal/miniapp)) — 손글씨 제출 등

진짜 Telegram/OpenAI 서버에는 붙지 않는다(외부 비결정성·비용·인증). 대신:
- **외부 텔레그램 API** = `mockBotAPI`(전송 메시지 캡처) — 이미 [test_common_test.go](../../internal/bot/test_common_test.go)에 존재, 재사용
- **외부 LLM** = 결정적 fake. bot 경유 시나리오는 같은 파일의 `mockLLM`(`gradeFn`·`answerFn`)을 재사용한다. `_test.go`는 다른 패키지에서 import할 수 없어 [grader_test.go](../../internal/service/grader_test.go)의 `mockLLM`은 bot·miniapp에서 쓸 수 없다. Mini App 시나리오는 `service.QuizGradingLLM`(`GradeAnswer`·`GradeHandwriting`)을 구현한 fake를 `package miniapp` 테스트에 새로 둔다.
- **DB** = ephemeral Postgres — `02_integration_test_plan.md`의 **testcontainers-go 하니스 재사용**(postgres:16-alpine 컨테이너 자동 기동). **2026-10-05 기준 미구현**이다: `go.mod`에 testcontainers가 없고, 현재 Postgres 테스트는 `COPYLINGO_TEST_DATABASE_URL`이 있을 때만 돌고 없으면 Skip한다. 02 하니스를 먼저 또는 함께 만든다.
- **Redis** = 격리된 Redis 컨테이너에 `redisstore` 구현을 연결한다. 기능별 저장 인터페이스의 in-memory fake는 단위 테스트에서 사용하며, 실제 키·직렬화·원자적 명령을 확인할 E2E에서는 실 Redis 저장 구현을 사용한다. 현재 redisstore 단위 테스트는 `redis.Cmdable` fake를 쓰므로 실 Redis 하니스도 없다. 02와 같은 testcontainers로 Redis 컨테이너를 추가한다.

즉 e2e = "외부 경계(텔레그램/LLM)만 mock, 내부(service+repo+DB+redis)는 전부 실물"로 시나리오를 관통한다.

---

## 1. 인프라

- 위치: **아래 "공용 하니스 — 패키지 배치와 조립" 참조.** bot 경유 시나리오는 mock Telegram을 끼우는 `newTelegramClient`가 unexported라 `internal/bot` 내부(`package bot`)에 둔다. HTTP 전용 시나리오는 `package miniapp`에 둔다.
- 빌드 태그: 모든 파일 최상단 `//go:build e2e`. 실행은 `go test -tags=e2e ./internal/bot/... ./internal/miniapp/...`.
- **Docker 데몬** 미가용 시 `t.Skip("docker unavailable")` (DB/Redis 컨테이너를 testcontainers가 띄우므로 사전 준비 불필요).

### 공용 하니스 — 패키지 배치와 조립

> 2026-10-05 ADR-059 B~E 완료 구조 기준. 조립 순서의 원본은 cmd/server [`newServices`](../../cmd/server/services.go)·[`initBot`](../../cmd/server/server.go)이다. bot 테스트 헬퍼 [`newTestBot`](../../internal/bot/test_common_test.go)은 `initBot`과 같은 방식으로 Flow를 조립한다(둘 중 하나를 바꾸면 다른 쪽도 맞춘다).

**배치를 정하는 제약**
1. **Telegram**: `bot.NewTelegramClient(token, debug)`는 `tgbotapi.NewBotAPI`로 토큰을 검증한다(네트워크). `mockBotAPI`를 끼우는 `newTelegramClient(api)`는 unexported다. Flow·Bot 생성자(`NewSessionFlow(SessionFlowDeps{…})`·`NewBot(BotDeps{…})`)는 exported지만 `Telegram` 필드를 채우려면 이 함수가 필요하므로 bot 경유 시나리오는 `package bot`에 둔다.
2. **LLM**: 더 이상 제약이 아니다. `service.NewSessionService(service.SessionDeps{…, LLM: …})`가 소비자 정의 인터페이스 `QuizGradingLLM`을 받으므로 fake를 production 변경 없이 넣는다. cmd/server `newServices`는 실 LLM client를 만들고 `package main`이라 쓰지 않는다.

**권장 배치**
- bot 경유(E2E-1·2·4): `internal/bot/e2e_*_test.go` (`package bot`, `//go:build e2e`)
- HTTP 전용(E2E-3): `internal/miniapp/e2e_handwriting_test.go` (`package miniapp`, `//go:build e2e`). initData 서명은 [auth_test.go](../../internal/miniapp/auth_test.go)처럼 `hmacSHA256`로 만들고, 화면 갱신 호출은 [handler_test.go](../../internal/miniapp/handler_test.go)의 `fakeHandwritingScreen`으로 기록한다.

**선행 작업 (테스트 코드만): `newTestBot`의 stores 인자 넓히기**

`newTestBot(api, stores *testInteractionStores, services *testServices)`는 in-memory fake 타입만 받아 `*redisstore.Interactions`를 넘길 수 없다. `newTestBot`과 그 안에서 쓰는 `newTestSessionFlow`·`newTestStudyFlow`·`newTestLLMQuestionFlow`의 stores 인자를 Flow store 계약(`quizInputStore`·`WordOrderDraftStore`·`HandwritingMessageStore`·`MiniAppRecoveryStore`·`QuestionTimingStore`·`studyInputStore`·`llmInputStore`·`inputClearer`)을 embed한 테스트 전용 인터페이스로 바꾼다. cmd/server가 같은 `interactions`를 모든 필드에 넘기므로 `*redisstore.Interactions`와 `*testInteractionStores` 모두 이 계약을 만족한다.
- e2e 전용 조립 함수를 따로 만들지 않는다. `initBot` 복제가 세 벌이 된다.
- typed-nil 주의: 인자가 인터페이스가 되면 nil `*testInteractionStores` 변수는 non-nil 인터페이스가 된다. 기존 호출부는 `nil` literal이나 `newTestInteractionStores()`만 넘기므로 그대로 둔다.
- `newTestBot`은 `PublicBaseURL`을 비워 둔다. 손글씨 WebApp 버튼이나 stale URL을 확인하는 시나리오(E2E-4)는 `newTestSessionFlow(api, nil, SessionFlowDeps{…store 필드 = interactions, PublicBaseURL: …})`로 SessionFlow를 직접 만든다.

```go
//go:build e2e

package bot // bot 경유 시나리오는 내부 패키지로 둘 것

// newE2ESystem(t, llm *mockLLM) → (b *Bot, api *mockBotAPI, db *sqlx.DB, rdb *redis.Client)
//  1. db  := 02 하니스: 실 Postgres + migrations/001_init.sql 적용 + truncate
//     rdb := Redis 컨테이너 연결 + FlushDB
//  2. repos := repository.NewRepositories(db)
//  3. session := service.NewSessionService(service.SessionDeps{
//         QuestionRepo:           repos.Question,
//         SessionRepo:            repos.Session,
//         SessionQuestionRepo:    repos.SessionQuestion,
//         QuizActiveSessionRepo:  repos.QuizActiveSession,
//         StudyActiveSessionRepo: repos.StudyActiveSession,
//         MaterialRepo:           repos.Material,
//         DB:                     db,
//         UserRepo:               repos.User,
//         Stores: service.SessionStores{
//             Quiz:  redisstore.NewQuizSessions(rdb),
//             Study: redisstore.NewStudySessions(rdb),
//         },
//         LLM: llm, // test_common_test.go의 mockLLM
//     })
//  4. api := &mockBotAPI{}
//     b := newTestBot(api, redisstore.NewInteractions(rdb), &testServices{
//         Session:            session,
//         User:               service.NewUserService(repos.User),
//         MaterialPreference: service.NewMaterialPreferenceService(repos.MaterialPreference),
//         Analyzer:           service.NewAnalyzerService(repos.User, repos.SessionQuestion),
//         // LLMQuestion은 필요한 시나리오만. Audio는 nil(TTS 없는 경로).
//     })
//  5. 세션 발송은 scheduler와 같은 경로: session.BuildForSlot(…) → b.sessionFlow.PushSession(ctx, chatID, sessionID, sessionType)
//     답변은 b.handleCallback / b.handleMessage로 update를 주입한다.
```

```go
//go:build e2e

package miniapp

// session := service.NewSessionService(service.SessionDeps{…위와 같음, LLM: handwriting fake})
// screen  := &fakeHandwritingScreen{}
// handler := NewHandler(HandlerDeps{
//     Session:           session,
//     Verifier:          NewInitDataVerifier(testBotToken, InitDataMaxAge),
//     HandwritingScreen: screen,
// })
// r := gin.New(); RegisterRoutes(r, handler)
// POST config.PathHandwritingSubmit — hmacSHA256로 서명한 initData + JSON strokes
```

> production 수정 없이 위 조립이 막히는 부분이 있으면 `## BLOCKED`에 기록하고, 우회 가능한 시나리오부터 완성한다.

---

## 2. 시나리오 (각각 하나의 Test 함수)

### E2E-1: 아침 세션 — 생성부터 완료까지 (가장 중요)
파일: `internal/bot/e2e_session_test.go`
1. seed: user 1명 + questions(객관식/주관식 섞어 N개)를 **DB에 실제 insert**
2. scheduler와 같은 경로로 시작: `SessionService.BuildForSlot`이 DB에서 문제를 뽑아 session + session_questions 생성 → `SessionFlow.PushSession`
3. 첫 문제 전송됨을 `mockBotAPI.sentMessages`로 확인(문제 본문 + inline keyboard)
4. 각 문제에 콜백/메시지로 답변 주입 → `SessionService`가 채점(mockLLM)하고 진행 상태를 Redis/DB에 기록
5. 마지막 문제 답변 → 세션 완료: status=completed, correct_count 반영, user streak +1
6. **DB 직접 조회로 최종 상태 검증**: sessions.status, sessions.correct_count, session_questions.is_correct, users.streak_days
7. mockBotAPI에 결과 요약 메시지가 전송됐는지 확인

> 이 테스트 하나가 SessionService(문제 선택→채점→진행 기록→완료)→repo→DB 전 구간의 회귀를 잡는다.

### E2E-2: 복습(Review) 세션 — SRS 경로
파일: `internal/bot/e2e_review_test.go`
1. seed: `user_question_progress.next_review_at`이 오늘/과거인 문제 몇 개 + 미래인 것 몇 개 (SRS 상태는 사용자별 테이블, ADR-035)
2. 복습 세션 시작 → due인 문제만 포함되는지(미래 due 제외) 검증
3. 정답/오답 답변 → SM-2 규칙대로 `user_question_progress`의 interval_days/ease_factor/repetitions/next_review_at이 갱신됐는지 **DB 조회로 검증**
4. 오답 문제는 다음 due가 가까워지고, 정답은 멀어지는지

### E2E-3: 손글씨 제출 (MiniApp HTTP 경로)
파일: `internal/miniapp/e2e_handwriting_test.go`(`package miniapp`, `//go:build e2e`)
1. seed: kana 손글씨 문제 1개 + 진행 중 세션 (DB 직접 insert)
2. `RegisterRoutes(r, miniapp.NewHandler(miniapp.HandlerDeps{…}))` 로 gin 엔진 구성 → `httptest.NewServer`/`ServeHTTP`로 `SubmitHandwriting` 엔드포인트 호출 — 유효 Telegram initData(올바른 HMAC 서명)와 JSON stroke 요청
3. `SessionService.SubmitHandwriting`이 fake LLM의 `GradeHandwriting`(결정적)으로 채점 → 200 + 결과 JSON (`service.HandwritingSubmitResult` 스키마 확인), `fakeHandwritingScreen`에 갱신 호출 기록
4. session_questions에 답변/정답 여부가 **DB에 기록**됐는지 검증
5. 인증 실패(위조 initData / 무서명) → 401, DB 변화 없음 (`InitDataVerifier.Verify` 경유)

### E2E-4 (선택): stale MiniApp 메시지 갱신
파일: `internal/bot/e2e_refresh_test.go`(`package bot`, `//go:build e2e`)
대상 [restart_recovery.go](../../internal/bot/restart_recovery.go) **`(*SessionFlow).RefreshStaleMiniAppMessages(ctx)`** (※ "세션 복구"가 아니라 stale miniapp 메시지 갱신 함수다):
1. seed: 진행 중 세션 + miniapp 버튼이 달린(이전 base URL 토큰) 메시지 상태를 Redis/DB에 구성
2. `RefreshStaleMiniAppMessages(ctx)` 호출 → 현재 base URL과 다른 stale 메시지가 갱신(재전송/edit)되는지 `mockBotAPI.sentMessages`로 검증
3. 이미 최신인 메시지는 건드리지 않는지

---

## 3. 결정성 확보 (flaky 방지)

- 시간 의존(streak, SRS due, today stats): 가능한 한 **DB에 명시적 날짜로 seed**해서 "오늘/어제/미래"를 제어. production의 `timeNow()`를 수정하지 말 것(테스트 전용 변경 금지). seed 날짜를 현재 기준 상대값으로 계산해 넣는다.
- LLM: 반드시 mock(고정 응답). 실제 OpenAI 호출 금지.
- 랜덤(`SessionService` 문제 선택의 "Random Slot Relay"): 결과 개수/구성에 대한 **느슨한 불변식**으로 검증(정확한 ID 순서 대신 "총 N개, 모두 unique, due 문제 포함" 식).
- 순서 의존 제거: 각 테스트는 truncate로 깨끗한 DB에서 시작.

---

## 4. 실행 & 검수 기준

```bash
# 전제: Docker 데몬만 떠 있으면 됨 (Postgres/Redis 컨테이너는 testcontainers가 자동 기동·정리)

go test -tags=e2e ./internal/bot/... ./internal/miniapp/...

# 일반 빌드 영향 없음 확인 (Docker 없어도 통과)
go test ./...
```

**완료 조건:**
1. `//go:build e2e` 로 옵트인 — `go test ./...` 에 영향 없음
2. 최소 E2E-1, E2E-2, E2E-3 세 시나리오 통과
3. 각 시나리오가 **입력 → 내부 전 계층 → DB 최종 상태**를 관통하고, 외부 경계(텔레그램/LLM)만 mock
4. 검증은 mockBotAPI 전송 메시지 + DB 직접 조회 **둘 다** 사용 (출력과 영속 상태를 모두 확인)
5. flaky 없음: 동일 명령 3회 연속 통과
6. production 코드 `git diff` 비어 있음 (`newTestBot` stores 인자 변경 같은 `_test.go` 변경은 허용)

production 수정 없이는 조립이 불가능한 경계(예: `bot.NewTelegramClient`가 네트워크 강제)는 하단 `## BLOCKED`에 `대상 — 막힌 이유 — 제안(테스트에서 우회한 방법 또는 필요한 production 변경)`로 기록하고, 우회 가능한 시나리오부터 완성한다.

---

## 5. 우선순위 요약 (세 문서 통합)

regression 방어 ROI 순서:
1. **02 integration / repository** — SQL·스키마 회귀(가장 자주 깨지는 곳). 최우선.
2. **01 unit / internal/bot** — 오케스트레이션 분기(현재 3.3%).
3. **03 e2e / E2E-1 아침 세션** — 핵심 동맥 1개.
4. 나머지 보강(service 잔여, miniapp, external, model, E2E-2~4).
