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
- **외부 텔레그램 API** = `mockBotAPI`(전송 메시지 캡처) — 이미 [handler_test.go](../../internal/bot/handler_test.go)에 존재, 재사용
- **외부 LLM** = `mockLLM`(결정적 응답) — 이미 [grader_test.go](../../internal/service/grader_test.go)에 존재, 재사용
- **DB** = ephemeral Postgres — `02_integration_test_plan.md`의 **testcontainers-go 하니스 재사용**(postgres:16-alpine 컨테이너 자동 기동)
- **Redis** = 둘 중 택1: ① testcontainers redis 모듈(`.../modules/redis`)로 실 Redis 컨테이너(완전 hermetic, 권장) ② in-memory fake `sessionFlowRedis`([session_flow_test.go](../../internal/bot/session_flow_test.go)) (가볍지만 Redis 동작 일부만 모사)

즉 e2e = "외부 경계(텔레그램/LLM)만 mock, 내부(service+repo+DB+redis)는 전부 실물"로 시나리오를 관통한다.

---

## 1. 인프라

- 위치: **아래 "공용 하니스 — 패키지 배치 주의" 참조.** bot 경유 시나리오는 `Bot`의 unexported 필드 때문에 `internal/bot` 내부(`package bot`)에 둬야 한다. HTTP 전용 시나리오만 `test/e2e/`(또는 `package miniapp`) 가능.
- 빌드 태그: 모든 파일 최상단 `//go:build e2e`. 실행은 `go test -tags=e2e ./internal/bot/... ./internal/miniapp/...` (또는 `test/e2e` 사용 시 그 경로 추가).
- **Docker 데몬** 미가용 시 `t.Skip("docker unavailable")` (DB/Redis 컨테이너를 testcontainers가 띄우므로 사전 준비 불필요).

### 공용 하니스 — **패키지 배치 주의**

> ⚠️ **확인된 제약 두 가지가 e2e 패키지 위치를 결정한다:**
> 1. `service.NewServices(repos, cfg, rdb)`는 내부에서 `external.NewLLMClient(cfg)`로 **LLM을 직접 생성**한다 → `NewServices`로는 mockLLM 주입 불가. **개별 생성자로 직접 조립**해야 한다(예: `service.NewGraderService(repos.User, activeSvc, mockLLM)`, `service.NewActiveSessionService(repos.ActiveSession, rdb, srs)` 등 — 시그니처는 services.go 참조).
> 2. `Bot` struct의 필드는 모두 **unexported**이고, `bot.New(...)`는 실제 텔레그램 토큰으로 `tgbotapi.NewBotAPI`를 호출(네트워크) → 외부 `test/e2e` 패키지에서는 mock api를 끼운 Bot을 만들 수 없다.
>
> **따라서 권장 배치:** bot 경유 시나리오(E2E-1/2/4)는 **`package bot` 내부에 `//go:build e2e` 파일**로 둔다(예: `internal/bot/e2e_session_test.go`). 그래야 `&Bot{api: &mockBotAPI{}, rdb: realRedis, services: 직접조립}` + `NewSessionFlow(b)` 가 가능하고 기존 `mockBotAPI`도 재사용된다. HTTP 전용 시나리오(E2E-3)는 `package miniapp` 내부 `//go:build e2e` 또는 `test/e2e`(핸들러가 exported라 가능)에 둔다.

```go
//go:build e2e
package bot   // bot 경유 시나리오는 내부 패키지로 둘 것

// buildSystem(t):
//  1. 실 Postgres 연결 + migrations 적용 + truncate (02 계획 하니스 로직 재사용/복제)
//  2. repos := repository.NewRepositories(db)
//  3. 서비스 "직접" 조립 (NewServices 쓰지 말 것 — LLM mock 주입 위해):
//        srs    := service.NewSRSService(repos.Question)
//        active := service.NewActiveSessionService(repos.ActiveSession, realRedis, srs)
//        grader := service.NewGraderService(repos.User, active, mockLLM)      // mockLLM = grader_test.go 재사용
//        builder:= service.NewSessionBuilderService(repos.Question, repos.Session, repos.SessionQuestion, srs)
//        svcs   := &service.Services{User: ..., SRS: srs, SessionBuilder: builder,
//                                    ActiveSession: active, Grader: grader, Handwriting: ..., Analyzer: ..., Tip: ...}
//  4. b := &Bot{api: &mockBotAPI{}, rdb: realRedis, cfg: &config.Config{...}, services: svcs}
//     sf := NewSessionFlow(b)
//  5. 반환: {b, mockAPI, sf, db, redis}
```

> production 수정 없이 위 조립이 막히는 부분이 있으면 `## BLOCKED`에 기록하고, 우회 가능한 시나리오부터 완성한다.

---

## 2. 시나리오 (각각 하나의 Test 함수)

### E2E-1: 아침 세션 — 생성부터 완료까지 (가장 중요)
파일: `test/e2e/session_flow_test.go`
1. seed: user 1명 + questions(객관식/주관식 섞어 N개)를 **DB에 실제 insert**
2. `/start` 또는 morning 세션 시작 트리거 → `SessionBuilder`가 DB에서 문제 뽑아 session + session_questions 생성
3. 첫 문제 전송됨을 `mockBotAPI.sentMessages`로 확인(문제 본문 + inline keyboard)
4. 각 문제에 콜백/메시지로 답변 주입 → `Grader`가 채점, `ActiveSession`이 Redis/DB에 기록
5. 마지막 문제 답변 → `CompleteSession`: 세션 status=completed, correct_count 반영, user streak +1
6. **DB 직접 조회로 최종 상태 검증**: sessions.status, sessions.correct_count, session_questions.is_correct, users.streak_count
7. mockBotAPI에 결과 요약 메시지가 전송됐는지 확인

> 이 테스트 하나가 SessionBuilder→Grader→ActiveSession→repo→DB 전 구간의 회귀를 잡는다.

### E2E-2: 복습(Review) 세션 — SRS 경로
파일: `test/e2e/review_flow_test.go`
1. seed: srs_due_date가 오늘/과거인 question 몇 개 + 미래인 것 몇 개
2. 복습 세션 시작 → due인 문제만 포함되는지(미래 due 제외) 검증
3. 정답/오답 답변 → `SRS.ProcessAnswer`로 questions.srs_interval/ease/repetitions/due_date가 규칙대로 갱신됐는지 **DB 조회로 검증**
4. 오답 문제는 다음 due가 가까워지고, 정답은 멀어지는지

### E2E-3: 손글씨 제출 (MiniApp HTTP 경로)
파일: `internal/miniapp/e2e_handwriting_test.go`(`package miniapp`, `//go:build e2e`) 또는 `test/e2e/`
1. seed: kana 손글씨 문제 1개 + 진행 중 세션 (DB 직접 insert)
2. `RegisterRoutes(r, cfg, services, rdb, messenger)` 로 gin 엔진 구성 → `httptest.NewServer`/`ServeHTTP`로 `SubmitHandwriting` 엔드포인트 호출 — 유효 Telegram initData(올바른 HMAC 서명) + 이미지 multipart
3. 내부 `Grader`가 mockLLM(결정적)으로 채점 → 200 + 결과 JSON (`SubmitHandwriting` 응답 스키마 확인)
4. session_questions에 답변/정답 여부가 **DB에 기록**됐는지 검증
5. 인증 실패(위조 initData / 무서명) → 401, DB 변화 없음 (`InitDataVerifier.Verify` 경유)

### E2E-4 (선택): stale MiniApp 메시지 갱신
파일: `internal/bot/e2e_refresh_test.go`(`package bot`, `//go:build e2e`)
대상 [restart_recovery.go](../../internal/bot/restart_recovery.go) **`(*Bot).RefreshStaleMiniAppMessages(ctx)`** (※ "세션 복구"가 아니라 stale miniapp 메시지 갱신 함수다):
1. seed: 진행 중 세션 + miniapp 버튼이 달린(이전 base URL 토큰) 메시지 상태를 Redis/DB에 구성
2. `RefreshStaleMiniAppMessages(ctx)` 호출 → 현재 base URL과 다른 stale 메시지가 갱신(재전송/edit)되는지 `mockBotAPI.sentMessages`로 검증
3. 이미 최신인 메시지는 건드리지 않는지

---

## 3. 결정성 확보 (flaky 방지)

- 시간 의존(streak, SRS due, today stats): 가능한 한 **DB에 명시적 날짜로 seed**해서 "오늘/어제/미래"를 제어. production의 `timeNow()`를 수정하지 말 것(테스트 전용 변경 금지). seed 날짜를 현재 기준 상대값으로 계산해 넣는다.
- LLM: 반드시 mock(고정 응답). 실제 OpenAI 호출 금지.
- 랜덤(SessionBuilder의 "Random Slot Relay"): 결과 개수/구성에 대한 **느슨한 불변식**으로 검증(정확한 ID 순서 대신 "총 N개, 모두 unique, due 문제 포함" 식).
- 순서 의존 제거: 각 테스트는 truncate로 깨끗한 DB에서 시작.

---

## 4. 실행 & 검수 기준

```bash
# 전제: Docker 데몬만 떠 있으면 됨 (Postgres/Redis 컨테이너는 testcontainers가 자동 기동·정리)

go test -tags=e2e ./internal/bot/... ./internal/miniapp/...   # (test/e2e 사용 시 그 경로도 추가)

# 일반 빌드 영향 없음 확인 (Docker 없어도 통과)
go test ./...
```

**완료 조건:**
1. `//go:build e2e` 로 옵트인 — `go test ./...` 에 영향 없음
2. 최소 E2E-1, E2E-2, E2E-3 세 시나리오 통과
3. 각 시나리오가 **입력 → 내부 전 계층 → DB 최종 상태**를 관통하고, 외부 경계(텔레그램/LLM)만 mock
4. 검증은 mockBotAPI 전송 메시지 + DB 직접 조회 **둘 다** 사용 (출력과 영속 상태를 모두 확인)
5. flaky 없음: 동일 명령 3회 연속 통과
6. production 코드 `git diff` 비어 있음

production 수정 없이는 조립이 불가능한 경계(예: `bot.New`가 네트워크 강제)는 하단 `## BLOCKED`에 `대상 — 막힌 이유 — 제안(테스트에서 우회한 방법 또는 필요한 production 변경)`로 기록하고, 우회 가능한 시나리오부터 완성한다.

---

## 5. 우선순위 요약 (세 문서 통합)

regression 방어 ROI 순서:
1. **02 integration / repository** — SQL·스키마 회귀(가장 자주 깨지는 곳). 최우선.
2. **01 unit / internal/bot** — 오케스트레이션 분기(현재 3.3%).
3. **03 e2e / E2E-1 아침 세션** — 핵심 동맥 1개.
4. 나머지 보강(service 잔여, miniapp, external, model, E2E-2~4).
