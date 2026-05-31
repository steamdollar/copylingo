# Unit Test 보강 계획 (for Gemini)

> 목표: regression(회귀)을 막기 위한 **unit test 공백 메우기**.
> **제약: 테스트 코드만 작성한다. production 코드는 절대 수정하지 않는다.**
> production 코드가 테스트 불가능하게 짜여 있어도 고치지 말 것 — 그 경우 해당 항목을 "BLOCKED: 이유"로 남기고 넘어간다.

---

## 0. 현재 상태 (측정값)

`go test -cover ./...` 기준:

| 패키지 | coverage | 비고 |
|---|---|---|
| internal/service | 78.5% | 양호 — 유지/소폭 보강 |
| internal/config | 86.7% | 양호 |
| internal/pipeline | 77.3% | 양호 |
| internal/miniapp | 58.2% | 보강 대상 |
| internal/external | 51.1% | 보강 대상 |
| **internal/bot** | **3.3%** | **최우선 보강** |
| internal/model | 0% | 로직 있는 타입만 |

이 문서는 **bot, service 잔여, miniapp, external, model** 의 순수 unit test를 다룬다.
DB·HTTP 실연동은 `02_integration_test_plan.md`, 전체 플로우는 `03_e2e_test_plan.md` 참조.

---

## 1. 작업 원칙

- **white-box 허용**: 기존 테스트가 `package bot`, `package service`, `package external`(동일 패키지)로 작성돼 있다. 같은 방식으로 internal 함수에 접근한다.
- **기존 mock 재사용**: 새 mock을 만들기 전에 아래 기존 mock을 먼저 확인하고 재사용/확장한다.
  - [internal/service/grader_test.go](../../internal/service/grader_test.go): `mockGraderUserRepo`, `mockGraderActiveSession`, `mockSRS`, `mockLLM`, 헬퍼 `activeStateForQuestion(...)`
  - [internal/service/session_builder_test.go](../../internal/service/session_builder_test.go): `mockQuestionFetcher`, `mockSessionStore`, `mockSessionQuestionStore`
  - [internal/bot/handler_test.go](../../internal/bot/handler_test.go): `mockBotAPI`(sentMessages 캡처), `mockRedis`
  - [internal/bot/session_flow_test.go](../../internal/bot/session_flow_test.go): `sessionFlowRedis`(in-memory Redis fake, Get/Set/Del 구현)
- **테이블 드리븐**: 가능한 한 `tests := []struct{...}` 패턴 + `t.Run` + `t.Parallel()`.
- **검증 대상은 분기와 에러 경로**: happy path 1개 + 각 에러/엣지 분기. regression은 보통 "조건 분기"에서 터진다.

---

## 2. internal/bot — 최우선 (현재 3.3%)

> ⚠️ **실제 코드 구조(확인됨, 반드시 이 이름 그대로 사용):**
> - `Bot` struct 필드(전부 unexported, 같은 `package bot` 테스트에서만 접근): `api BotAPI`, `cfg *config.Config`, `services *service.Services`, `rdb redis.Cmdable`, `flow *SessionFlow`, `stopCh chan struct{}`.
>   → `miniAppURL`, `stateStore`, `contentService` 같은 필드는 **없다**. 만들지 말 것.
> - 답변/문제 진행 로직은 `Bot`이 아니라 **`*SessionFlow`** 메서드에 있다. `SessionFlow`는 `NewSessionFlow(bot *Bot) *SessionFlow` 로 생성.
> - 전송 mock은 **`BotAPI`(exported 인터페이스)**. 기존 `mockBotAPI`(handler_test.go)가 이미 이를 만족.

Bot 직접 조립 예 (production 수정 불필요):

```go
b := &Bot{
    api:      &mockBotAPI{},        // BotAPI 만족, sentMessages 캡처 (handler_test.go)
    rdb:      &sessionFlowRedis{values: map[string]string{}}, // session_flow_test.go fake; redis.Cmdable 미충족 시 mockRedis 확장
    cfg:      &config.Config{},
    services: &service.Services{ /* 필요한 서비스만 service.NewXxxService(mockdeps...) 로 주입 */ },
}
sf := NewSessionFlow(b)
```

> `services`의 각 필드는 구체 타입(`*GraderService` 등). 그 의존성을 mock으로 넣어 `service.NewGraderService(mockUserRepo, mockActive, mockLLM)` 식으로 만들어 끼운다. session_flow_test.go가 `service.NewActiveSessionService(nil, rdb, nil)`를 끼우는 방식 그대로.

### 신규 파일: `internal/bot/session_answer_test.go`
대상: [session_answer.go](../../internal/bot/session_answer.go) — `(*SessionFlow).processAnswer`, `HandleTextInput`, `processAnswerText`

- `processAnswer`: 객관식 선택 → Grader 채점 호출, 결과 메시지/다음 문제 전송(`mockBotAPI.sentMessages` 검증)
- `processAnswer`: 이미 답한 문제(`ErrActiveSessionAlreadyAnswered`) → 중복 채점 안 함
- `processAnswerText`: 주관식 정답/오답/`ErrAIUnavailable`(AI 미가용 안내 메시지) 분기
- `processAnswerText`: `editMessageID`가 nil일 때(신규 전송) vs 값 있을 때(edit) 동작 차이
- `HandleTextInput`: 진행 중 세션이 텍스트 입력을 기다릴 때 true 반환/처리, 아닐 때 false 반환(소비 안 함)

### 신규 파일: `internal/bot/session_question_test.go`
대상: [session_question.go](../../internal/bot/session_question.go) — `showQuestion`, `renderByType`, `isQuestionAnswered`, `nextUnansweredQuestionIndex`, `handwritingMiniAppURL`, `isStaleMiniAppCallback`

- `renderByType` 객관식 → inline keyboard(선택지 버튼) 포함 메시지
- `renderByType` 손글씨(`model.QuestionKanaHandwriting`) → miniapp 버튼/URL 포함
- `renderByType` 주관식 → 텍스트 입력 안내
- `nextUnansweredQuestionIndex` / `isQuestionAnswered`: 기존 session_flow_test.go에 일부 검증 있음 → 미답/전부답함/범위밖 케이스 보강
- `handwritingMiniAppURL`: language/level/세션·문제 ID가 URL에 올바로 인코딩, base URL 빈 경우 처리
- `isStaleMiniAppCallback`: legacy(토큰 없음)=stale, 동일 토큰=not stale, 다른 토큰=stale (callback 패키지 버전과 동작 일치 확인)

### 신규 파일: `internal/bot/session_helpers_test.go`
대상: [session_helpers.go](../../internal/bot/session_helpers.go) — 순수 함수 위주, 빠른 regression 가드
- `formatSessionAnswer(userAnswer *string)`: nil → 미답 표기, 값 있으면 그 값
- `sessionTypeLabel(t string)`: morning/evening/review/article 각 라벨 + 미지정 fallback
- `truncate(s, maxLen)`: maxLen 이하/초과/멀티바이트(한·일 문자) 경계 — 잘림 안전성
- `stripHTML(s)`: 태그 제거, 중첩/미완성 태그 안전 처리
- `mainMenuKeyboard()`: 기대한 버튼 구성 반환

### 신규 파일: `internal/bot/handler_dispatch_test.go`
대상: [handler.go](../../internal/bot/handler.go) — `handleUpdate`, `handleMessage`, `handleCallback`, 커맨드 핸들러들
- `handleUpdate(update)`가 message/callback/그 외를 올바른 핸들러로 라우팅(시그니처: `handleUpdate(update tgbotapi.Update)`, ctx 파라미터 없음 — 내부에서 생성)
- 커맨드 분기: `handleStart`/`handleMenu`/`handleStats`/`handleStreak`/`handleHelp`/`handleExit`/`handleTest` 각 진입 (메시지 `Command()`별)
- `handleHelp` → 도움말 텍스트 전송 (의존성 적어 가장 쉬움, 여기부터 시작 권장)
- `handleExit` → 기존 handler_test.go에 검증 있음, 유지
- `handleCallback` 콜백 data prefix 분기(`q:` 등)가 `SessionFlow`로 위임되는지
- `languageDisplayName(code)` 순수 함수: 알려진 코드/미지의 코드 fallback

### 보강: `internal/bot/session_flow_test.go` (기존 파일에 추가)
대상: `StartStudy`, `StartReview`, `startSession`, `finishSession`, `getPendingSessions`, `getInProgressSessions`
- `startSession`: 세션 로드 성공 → 첫 문제 전송 / 활성 세션 불가 → `showActiveSessionUnavailable` 경로
- `finishSession`: `CompleteSession` 결과로 요약 메시지(정답수/총수) 전송 구성
- `getInProgressSessions`: 진행 중 세션 있음/없음 분기

> 참고: `restart_recovery.go`의 함수는 `RefreshStaleMiniAppMessages(ctx)`(진행중 세션 복구가 아니라 **stale miniapp 메시지 갱신**)이다. DB/외부 의존이 크면 unit보다 integration이 맞으니, 분기만 unit으로 가능하면 하고 아니면 BLOCKED 표기 후 `02`/`03`으로 미룬다. `webapp.go`는 `newWebAppButton`/`newCallbackButton` 빌더뿐이라 간단한 unit 가능.

---

## 3. internal/service — 잔여 보강 (현재 78.5%)

대부분 덮였으나 다음 파일이 미검증이거나 약하다. `go test -coverprofile`로 미커버 라인 확인 후 보강:

### 신규 파일: `internal/service/tip_test.go`
대상: [tip.go](../../internal/service/tip.go) — `tipRepository` 인터페이스 mock 작성(Create/ListActive/CountActive 등 실제 시그니처 확인 후)
- 캐시/조회 로직, 빈 결과, repo 에러 전파

### 보강: 기존 `*_test.go`의 미커버 분기
- [active_session.go](../../internal/service/active_session.go): Redis 캐시 miss → DB load fallback, 버전 불일치 처리, Flush 후 Delete 순서
- [handwriting.go](../../internal/service/handwriting.go): tip 캐싱/조건부 검증 경로(최근 커밋·workthrough 문서 참조: `docs/workthrough/2605/2605301343_handwriting_conditional_verification.md`)
- [srs.go](../../internal/service/srs.go): `ProcessAnswer` 정답/오답 시 interval·ease·repetitions 갱신 공식 경계값(ease 하한, interval 0→1 등)

---

## 4. internal/external — 보강 (현재 51.1%)

기존 [llm_test.go](../../internal/external/llm_test.go), nhk_client_test.go가 `httptest.Server`로 HTTP를 mock. 같은 패턴으로 미커버 분기 보강:
- `llm.go`: API 키 미설정 → `ErrAIConfigMissing`; HTTP 5xx/timeout → 에러 래핑; 응답 JSON 파싱 실패; GradeAnswer/GradeHandwriting/Translate 각각의 정상/이상 응답
- `nhk_client.go`: 비정상 status, 빈 body, 파싱 실패

> 이건 in-process httptest라 "narrow integration"에 가깝지만, 기존 컨벤션상 같은 패키지 unit으로 둔다.

---

## 5. internal/model — 로직 있는 타입만 (현재 0%)

순수 구조체는 skip. **메서드/로직이 있는 타입만** 테스트:
- [active_session.go](../../internal/model/active_session.go): `ActiveSessionStateVersion` 관련 버전 체크 메서드, 진행도 계산 등 메서드가 있으면 테스트
- [question.go](../../internal/model/question.go): `QuestionType` 상수 분기, Options(JSONB) 직렬화 헬퍼가 있으면 테스트
- 메서드 없는 파일(stats.go 등 순수 struct)은 작성하지 않는다.

먼저 `grep -nE "^func \(" internal/model/*.go` 로 메서드 존재 여부를 확인하고, 없으면 해당 타입은 생략.

---

## 6. 실행 & 검수 기준

```bash
# 전체 테스트 통과
go test ./...

# 패키지별 커버리지 재측정 (목표: bot 3.3% → 50%+ )
go test -cover ./...

# 미커버 라인 시각 확인 (특정 패키지)
go test -coverprofile=/tmp/c.out ./internal/bot/...
go tool cover -func=/tmp/c.out
```

**완료 조건:**
1. `go test ./...` 전부 통과 (기존 테스트 포함, 깨뜨리지 말 것)
2. production 코드 `git diff` 가 비어 있음 (테스트 파일만 추가/수정)
3. internal/bot coverage ≥ 50%
4. 새 mock은 기존 것을 재사용하거나 같은 스타일로 작성
5. 각 테스트는 분기 1개 이상을 명확히 검증 (단순 호출만 하는 빈 테스트 금지)

production 수정이 필요해 막힌 항목은 파일 하단 `## BLOCKED` 섹션에 `대상함수 — 필요한 변경 — 이유`로 기록.
