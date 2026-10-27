# 과잉 추상화·테스트 가치 검토 후속 (A~I)

> 2026-10-26 Case 0 검토 결과. 질문은 "불필요한 interface와 과잉 추상화가 있는가, 테스트 때문에 생긴 것이라면 그 테스트가 단순성을 포기할 만한가"였다.
> 줄 번호는 커밋 `f422aeb` 기준이다. 어긋나면 심볼로 다시 찾는다.
> 항목마다 독립 커밋이다. 결정 상태는 각 항목의 **상태** 줄을 따른다. 사용자가 하나씩 검토한 뒤 확정한다.

## 판단 기준

- `AGENTS.md` §4: 수만 명 규모를 가정한다. YAGNI도, 근거 없는 패턴 고정도 기본값이 아니다.
- ADR-059의 큰 틀은 유지한다. 소비자 정의 interface, Tier1 공개 interface, 테이블 단위 repo, `SessionDeps`가 여기에 해당한다.
- 정리 대상은 그 아래 층이다. 운영 구현이 1개이고 테스트 fake만 끼우려고 만든 Tier2 하위 interface, 운영에서 도달할 수 없는 nil 분기, 동작을 보호하지 않는 테스트다.
- 테스트를 지우는 기준: 문구·리터럴 고정, 인자 전달만 확인, 다른 테스트와 중복, 테스트 전용 helper를 테스트하는 경우.

## 기존 TODO와의 관계 (`structure_review_followups.md`)

| 이 문서 | 기존 항목 | 처리 |
|---|---|---|
| B (`studyActiveSessionStarter` 제거) | U4 "Study Tier1이 전달만 함" | U4에 포함돼 있다. B에서는 나머지 하위 interface만 다룬다 |
| B (SRS를 순수 함수로) | U4 "SRS 정책 2벌·반올림 불일치"(ADR 필요) | B는 interface와 전달 계층만 제거한다. 정책 단일화와 반올림은 U4 결정을 따른다 |
| C (grader concrete화) | U4 "grader 채점과 기록 분리" | 같은 작업으로 합쳐 완료했다. Redis 이중 Load는 의도적으로 남겼다(C 참고) |
| E (SessionFlow store 1개로) | U3 "interactions.go 병합 후 unexport" | E를 하면 U3 항목도 같이 끝난다 |
| F (bot 테스트 삭제) | U3 "coverage_boost 해체·중복 삭제" | **F를 U3보다 먼저 한다.** 지울 테스트를 옮기는 수고를 없앤다 |
| G (pipeline 삭제) | "손대지 말 것: ADR-057 pipeline 보존" | G는 이 결정을 다시 여는 항목이다. 승인 전까지는 기존 결정이 유효하다 |
| H (callback 패키지를 bot에 병합) | U3 "`session_flow_test.go:13-131`을 `callback_test.go`로" | H를 하면 U3 항목이 무의미해진다. 둘 중 하나만 한다 |
| H (`sessionRedis`) | U3 "`sessions.go`·`session_store.go` 병합" | 같은 diff에서 처리할 수 있다 |

**권장 순서**: (A·C 완료) → B → (D + E) → F → U3 rename → H. G는 별도 결정.

---

## A. [완료] `ErrAIConfigMissing`이 운영에서 발생하지 않음

- **결과 (ADR-083)**: guard를 고치는 대신 LLM key를 필수로 바꿨다. key가 없으면 서버가 시작하지 않는다. 그래서 `ErrAIConfigMissing`·`ErrAIUnavailable`·guard 3곳·Mini App 503 매핑과, 문항이 0건이던 subjective 타입(LLM 의미 채점·오답 처리 fallback 포함)을 함께 지웠다. key가 있지만 무효한 경우는 런타임 에러로 처리한다.

## B. [완료] service Tier2 하위 interface 제거

- **상태**: 완료(`srsService` 삭제, `StrokeRenderer` 이동, `sessionStore`·`studySessionStore` → `SessionRepo`, builder 테스트 정리). ADR-059 §8.4가 Tier2끼리는 concrete 호출을 허용하므로 ADR 수정은 필요 없다.
- **대상과 근거**:

| interface | 위치 | 운영 구현 / fake | 문제 |
|---|---|---|---|
| ~~`srsScheduler`, `questionQuerier`, `questionFetcher`~~ | **완료** | - | `srsService`를 지웠다. builder는 `QuestionRepo.GetDueReviews`를 직접 부르고, 복습 개수는 `SessionService.DueReviewCount`가 repo를 직접 부른다. SM-2는 순수 함수 `scheduleAnswer` |
| ~~`sessionStore`, `studySessionStore`~~ | **완료** | - | builder·study 생성자가 `SessionRepo`를 직접 받는다. fake는 nil `SessionRepo`를 embed하고 쓰는 메서드만 구현한다 |
| ~~`StrokeRenderer`~~ | **완료** | - | 렌더러를 `internal/miniapp`으로 옮기고 interface를 지웠다. service는 렌더링된 PNG(`HandwritingSubmitRequest.Image`)만 받는다 |

- **변경**:
  - Tier2 생성자는 Tier1 interface(`QuestionRepo`·`SessionRepo`)를 받는다. 테스트 fake는 이미 쓰는 embed 패턴(`session_quiz_test.go:13`)으로 필요한 메서드만 구현한다.
- **테스트**:
  - (완료) `srs_test.go`의 전달 테스트 5개를 지웠다. level 범위 검증은 `TestSessionServiceDueReviewCountUsesSessionLevels`와 builder 테스트의 `gotLevels` 검사로 옮겼다. `mockSRS`는 `mockDueReviews` + `builderQuestionRepo`로 바꿨다.
  - (완료) `mockSessionStore`의 죽은 메서드 `GetSessionsByStatus`·`ListInProgress`·`Start`를 지웠다.
  - (완료) builder 테스트 3파일(2,669줄 → 1,479줄): 8인자 closure fake 4종을 in-memory `builderPool`(QuestionRepo, 호출 인자 기록)과 `builderStores`(저장 결과 기록)로 합쳤다. 시나리오 19+2개는 유지하고 closure 안 단언은 build 후 기록 검사로 옮겼다. production 변이 8종(reading·kanji·maintenance cap, dedup, neighbor 순서, reserved due, due LIMIT, adjacent fallback)이 모두 실패로 잡히는 것을 확인했다.
- **LOC**: builder 테스트 실측 −1,190줄.

## C. [완료] `graderQuizActiveSession` concrete화 (U4 "grader 채점과 기록 분리"와 합침)

- **결과**: `graderService`와 interface를 지웠다(`grader.go` 삭제).
  - 텍스트·선택지 답안은 `gradeQuizAnswer`에서 정답 문자열과 바로 비교한다.
  - 손글씨는 `SubmitHandwriting`이 LLM을 직접 호출한다.
  - 기록은 두 경로 모두 `SessionService`가 `quizProgress.RecordAnswer`를 직접 호출한다.
  - 항상 통과하던 질문 일치 검사와 중복된 handwriting 타입 검사도 함께 사라졌다.
- **남긴 것: Redis 이중 Load**: `RecordAnswer`는 저장 전에 state를 다시 읽는다.
  - Load를 1회로 줄이면 손글씨 LLM 채점(수 초) 동안 읽은 state로 저장하게 된다. 그 사이 다른 기록이나 세션 정리를 덮어쓸 수 있다.
  - 이득은 Redis GET 1회다. 위험에 비해 작아서 유지했다.
- **테스트**: `grader_test.go`를 삭제했다.
  - 공용 mock은 옮겼다. `mockSRS`는 `session_builder_test.go`로, `mockLLM`은 `session_quiz_handwriting_test.go`로 갔다.
  - `activeStateForQuestion`은 `quiz_active_session_test.go`로 옮겼다.
  - 저장 실패 전파는 제출 경로 테스트(`TestSessionServiceSubmitQuizOptionSaveFailure`)로 대체했다.

## D. [완료] 도달할 수 없는 nil 방어 분기 삭제

- **결과**: ADR-084로 ADR-059 §8.7의 "store nil 분기 유지"를 대체했다. 운영 조립이 항상 채우는 의존의 nil 분기 37곳과 `Err{Quiz,Study}ActiveSessionDependencyMissing`을 지웠다.
  - service 12곳(quiz 5·study 7; `grader.go`는 C에서 이미 삭제)
  - bot store 15곳
  - scheduler 7곳: quiz/study pusher, limiter, `rateLimiter` nil receiver 2곳, `ratePerSec <= 0`, `claims`. `claims`도 운영에서 항상 채워져 같은 기준으로 지웠다.
  - Mini App `handwritingScreen` 1곳
  - 청해 음성 `audio == nil` 1곳
- **테스트**:
  - 분기만 고정하던 테스트 6개를 지웠다(service 5, Mini App `TestRefreshHandwritingMessagePreservesParentCorrelation`).
  - nil repo를 "DB에 세션 없음"으로 쓰던 테스트는 `sql.ErrNoRows` fake로 바꿨다. bot의 `newTestSessionService`는 Quiz repo 기본값을 not-found fake로 둔다.
  - scheduler `mockPushClaims`는 user·slot·날짜별로 한 번씩 claim하고, 동시 호출에 안전하다.

## E. SessionFlow store 필드 4개를 1개로

- **상태**: 권장. U3 "interactions interface unexport"를 함께 끝낸다.
- **문제**: [session_flow.go:119-133](../../internal/bot/session_flow.go#L119-L133)의 `Input`·`Drafts`·`Messages`·`Recovery`에 [server.go:43-46](../../cmd/server/server.go#L43-L46)이 모두 같은 `interactions`를 넣는다. ADR-059가 `bot.StateStores`를 지운 이유("다섯 필드가 모두 같은 interactions")와 같은 패턴이다. 테스트에서 필드를 따로 바꾸는 곳은 `fingerprintSpy`(`util_recovery_test.go:72`) 하나뿐이고, `*testInteractionStores`를 embed하는 방식으로 바꿀 수 있다.
- **변경**:

```go
// before
type SessionFlowDeps struct {
	...
	Input    quizInputStore
	Drafts   WordOrderDraftStore
	Messages HandwritingMessageStore
	Recovery MiniAppRecoveryStore
	...
}

// after: interactions.go의 export interface 3개 + quizInputStore를 하나로 합친다
type sessionFlowStore interface {
	quizInputStore
	// WordOrderDraftStore·HandwritingMessageStore·MiniAppRecoveryStore의 메서드
}

type SessionFlowDeps struct {
	...
	Store sessionFlowStore
	...
}
```

- **LOC**: impl 약 −20 / test 약 −10.

## F. 가치 낮은 테스트 삭제

- **상태**: 권장. 항목별로 빼도 된다. **U3의 bot 테스트 재배치보다 먼저 한다.**
- **대상**:

| 위치 | 테스트 | 이유 | 처리 |
|---|---|---|---|
| `external/llm_test.go:14-113` | handwriting prompt 테스트 5개 | prompt 문장 약 35개를 `strings.Contains`로 고정한다. prompt를 고칠 때마다 같이 고쳐야 하고 LLM 판정은 검증하지 못한다 | 삭제. `:115`(user prompt에 Expected Text)와 `:133`(JSON schema)은 유지 |
| `external/llm_test.go:24` | `TestNewLLMClient` | 필드가 nil이 아닌지만 본다 | 삭제 |
| `config/config_test.go:49,71` | 기본값 테스트 2개 | `setDefaults()` 리터럴을 다시 적었다. env override 테스트는 bindEnv 누락을 잡으니 유지 | 삭제 |
| `bot/handler_dispatch_test.go:288` | `TestHandleHelp` | "도움말" 문자열만 본다 | 삭제 |
| `bot/coverage_boost_test.go:824` | `TestHandleStreak` | "스트릭"만 보고 숫자는 안 본다 | 삭제 또는 streak 값 assert |
| `bot/coverage_boost_test.go:966` | `TestHandleMenu` | "CopyLingo"만 본다. 준비한 count 5를 아무도 검사하지 않는다 | 삭제 또는 count assert |
| `bot/session_flow_extended_test.go:699` | `TestPushSession` | 문구만 본다 | 삭제 또는 callback data assert |
| `bot/coverage_boost_test.go:1012` | `TestTelegramClientEditMessageReplyMarkup` | tgbotapi 생성자가 만드는 타입만 본다 | 삭제 |
| `bot/handler_dispatch_test.go:191` | `TestLanguageDisplayName` | map 상수 확인 | 삭제 |
| `bot/handler_test.go:54` | `TestHandleExit` | 문장 전체 비교 | 입력 삭제 assert만 남김 |
| `bot/coverage_boost_test.go:433` | `TestShowQuestion_MultipleChoiceKeyboard` | `TestBuildMCQLayout` "short options" + `TestRenderByType/MultipleChoice`와 중복 | 삭제 |
| `bot/coverage_boost_test.go:554` | `TestShowQuestion_SubjectivePrompt` | `TestRenderByType/Subjective`와 중복 | 삭제 |
| `service/session_query_test.go:57-129` | 전달 테스트 3개 | 인자 전달만 본다 | 삭제 |
| `service/user_test.go` | `TestGetUser`, `TestGetAllUsers` | 전달만 본다 | 삭제 |
| `service/quiz_active_session_test.go` | `TestQuizActiveSessionCreateFromDB_ShufflesQuestionOptions`(약 110줄) | `model.TestQuestion_ShuffleOptions`와 중복 | 삭제 |
| `scheduler/dispatcher_test.go:239` | `TestDispatcher_RateLimiter` | ticker가 nil을 반환하는지만 본다 | 삭제 |
| `cmd/server/app_test.go:16-50,206-211` | 종료 순서 probe | TCP dial로 "scheduler stop 시점에 HTTP closed"를 고정한다. 이 순서의 근거가 ADR에 없다 | **owner 확인 후** 삭제. 시작 여부와 refresh ctx 취소 검사는 유지 |

- **고칠 것 (삭제 아님)**: `TestSessionServiceBuildForSlot`(`session_study_test.go:183`)은 아침·저녁 Quiz가 둘 다 due limit 19라서 둘을 구분하지 못한다. 서로 다른 값을 넣어 구분한다.
- **유지**: `question_repo_test.go:259-330`의 SQL 문자열 고정 테스트. Postgres 테스트는 `COPYLINGO_TEST_DATABASE_URL`이 없으면 skip되므로 `make test` 기본 실행에서 남는 유일한 보호막이다.
- **LOC**: test 약 −800.

## G. pipeline / NHK / ContentService 삭제

- **상태**: **결정 필요. ADR-057 재논의(Case A).** ADR_from_41_to_60 `:166`(2026-10-05 보강)이 "`pipeline` 패키지·`ContentService`·NHK client는 보존한다"고 정했다.
- **근거**:
  - `internal/pipeline`을 import하는 곳은 `internal/` 밖에 없다(`import_boundary_test.go:32` 표에만 있음).
  - Fetcher/Processor/Saver 3단계 plug-in 구조인데 각각 구현이 1개다. Processor는 그냥 통과만 시킨다. Orchestrator에 등록된 pipeline은 0개다.
  - `WithMaxArticles`·`WithHTTPClient` option은 테스트에서만 쓴다. 로그는 아직 `log.Printf("[X] INFO:")` 형식이다.
  - `repos.Content`는 생성만 하고 쓰는 곳이 없다.
- **선택지**:
  - A(권장) 삭제. git history가 보존 역할을 한다. 다시 쓸 때는 concrete 함수 하나로 충분하다. 삭제 결정과 이유를 ADR에 남긴다(§4: 추가하지 않는 결정도 ADR 대상).
  - B 현행 유지. 비용은 죽은 코드 약 1.1k LOC와 그 테스트를 계속 관리하는 것이다.
- **변경(A 선택 시)**: `internal/pipeline/`, `internal/external/nhk_client.go`·테스트, `service/content.go`·테스트, `repository/content_repo.go`, `Repositories.Content`, import allowlist의 pipeline 행. ADR-057 보강과 `structure_review_followups.md`의 "손대지 말 것" 줄도 수정한다.
- **LOC**: impl 약 −590 / test 약 −550.

## H. 자잘한 정리 (각각 독립, 안 해도 됨)

| 대상 | 위치 | 변경 | LOC |
|---|---|---|---|
| `callback` 패키지 | `internal/callback/callback.go` | importer가 bot 하나뿐이다(miniapp 사용은 `3723725`에서 제거). `bot/bot_constants.go`로 합치고 import allowlist 행을 지운다. U3의 callback_test 이동 항목과는 둘 중 하나만 한다 | −5, 패키지 −1 |
| `sessionRedis` | `redisstore/session_store.go:17` | 형제 store(`Interactions`·`PushClaims`)처럼 `redis.Cmdable` + embed fake로 통일한다 | −17 |
| level 검증 중복 | `config/config.go:98`, `observability/logger.go:69` `ParseLevel` | `ParseLevel`(exported지만 내부에서만 사용)을 unexport하거나 `slog.Level.UnmarshalText`로 대체한다 | −15 |
| 죽은 seam·코드 | `repository/repositories.go:39` `timeNow()`, `user_repo.go:81-82` `now := "now()"; _ = now` | `timeNow`는 교체할 수 없는 "testability용" 함수다. `time.Now()`로 바꾸고 죽은 줄은 지운다 | −8 |
| 운영 사용 0 helper | `model.SkillPtr`, `model.NewUserQuestionProgress`, `StudyActiveSessionState.ItemAt` | helper와 그 테스트(`TestSkillPtr`, `TestNewUserQuestionProgress`)를 같이 지운다 | −15 / test −30 |
| repository DB shim | `material_repo.go:16` `materialDB`, `material_preference_repo.go:14` `materialPreferenceDB` | Postgres 조건부 테스트가 `*sqlx.Tx`를 넘기려고만 존재한다. `sessionRowQueryer`(운영에서 DB와 Tx 모두 받음)와 함께 패키지 공용 `dbtx` 하나로 합친다 | −15 |
| store interface 복제 | `service/quiz_active_session.go:20` `QuizSessionStore`, `study_active_session.go:20` `StudySessionStore` | 운영은 이미 generic `redisstore.SessionStore[T]`다. `SessionStateStore[T]` 하나와 generic fake 하나로 합친다 | −15 / test −40 |
| bot interface 중복 | `material_preference.go:19,35` `materialPreferences`·`materialPreferenceList`, `settings.go:17` `settingsUser` | 앞의 둘은 같은 구현체라 하나로 합친다. `settingsUser`는 `userReader`를 embed한다 | −17 |
| Telegram 전송 중복 | `telegram_client.go:139` TODO | `SendMessage`가 `SendMessageWithKeyboard`를 호출하게 한다 | −10 |
| scheduler 주입 | `scheduler.go:94` `Deps.Cron`, `dispatcher.go:41` `workers`(항상 4) | 같은 패키지 테스트라 `s.cron`에 직접 접근할 수 있다. cron은 `New` 안에서 만들고 `workers`는 상수로 바꾼다 | −10 |

## I. 테스트 공백 (추가 작업, 단순화와 별개)

정리하다 발견한 것이다. 테스트가 많아도 핵심 경로가 비어 있다.

- **scheduler**: `tick`(timezone·slot 계산, backlog 집계, audio 트리거)과 `pushBuiltSession`(study/quiz 라우팅)의 coverage가 0%다.
  - 원인: `dispatcher_test.go:53`의 `schedulerSessionQueryRepoStub`이 nil `service.SessionRepo`를 embed하고 실제 `service.NewSessionService`를 조립한다. 이미 있는 2-method interface `dispatchSessions`(`dispatcher.go:22`)를 fake하지 않아 service 내부 구현에 묶여 있다. 테스트 4개가 모두 `unfinishedCount=3`이라 remind 경로만 지난다.
  - 변경: `dispatchSessions`·`slotUsers` fake로 바꿔 scheduler 테스트의 service import를 없애고, tick과 build→push 테스트를 추가한다.
- **restart recovery**: `util_recovery_test.go`의 3개는 모두 조기 종료 경로(base URL 없음, 세션 없음, ctx 취소)만 본다. 핵심 경로(fingerprint 불일치 → 키보드 제거 → 문제 재전송 → fingerprint 저장)는 테스트가 0개다. 조기 종료 테스트 2개를 핵심 경로 테스트 1개로 바꾼다. 그러면 `fingerprintSpy`도 필요 없다(E와 연결).

## 보류 (별도 결정)

- **bot 테스트 조립 방식**: bot 테스트는 bot interface 대신 실제 `*service.X`(`test_common_test.go:226`)와 repo fake 27종(약 790 LOC, 가짜 SQL driver 포함)을 조립한다. 단순 전달 의존(user·audio·analyzer·llmQuestion)만 bot interface로 직접 fake하면 test 약 −300이다. `SessionService`·`MaterialPreferenceService`는 상태 전이와 pagination 로직이 있으니 실제 객체를 유지한다. 바꾸는 범위가 넓어서 따로 결정한다.
- **miniredis 도입**: Redis fake 3개(약 250 LOC)를 실제 Redis 동작으로 대체한다. test 전용이지만 third-party 의존이 늘어난다.
- Study Tier2 흡수는 U4에 이미 있다.

## 유지 판정 (확인 완료, 재검토 불필요)

- service: `session.go`의 Tier1 interface 8개, `SessionDeps`·`SessionStores`, audio 외부 경계 3종, analyzer·user·materialPreference·LLM 소비자 interface, `sessionBuilderService`·`quizActiveSessionService`(실제 로직 있음), `testutil.TransactionDB`, `OnAIGrading`(운영 기능)
- bot·miniapp: Flow별 interface(안 쓰는 메서드 0개), `TelegramClient`(전송 로그·HTML 정리를 한곳에서), `BotAPI`, miniapp `quizSession`·`verifier`·`handwritingScreen`, `webapp.go` struct(tgbotapi v5.5.1에 web_app 버튼 없음), `InitDataVerifier.now`
- 인프라: scheduler interface 6개(import 경계상 필요), `cmd/server/app.go` interface 3개(포트 충돌 시 아무것도 시작하지 않는 동작 테스트), `observability.ContextHandler`·`DailyWriter`, external `*Options` struct(하위 계층의 config import 금지), TTS `transcode`, `SessionStore[T]`, `bootstrap`, `config/constants.go`
- 테스트: `import_boundary_test`, `logging_test`(메시지 본문 노출 방지), `sanitize_test`(Telegram `<br>` 거부), `cloneTestState`, `cmd/seeding` 테스트

## 검증

- 항목마다 `make test`. Go 파일을 고치면 `goparams`를 적용한다(`docs/CONVENTIONS.md#agent-go-formatting`).
- A·C·D는 런타임 동작이 바뀌므로 `make restart-app` 후 `/health`를 확인한다.
- A는 API key를 비운 설정으로 주관식 채점을 실행해 AI 미설정 안내가 나오는지 확인한다.
- C는 답 제출 1회에 Redis Load가 1회인지 확인한다(fake store 카운터).

## 손대지 말 것

- ADR-059의 큰 틀(소비자 정의 interface, Tier1 공개 interface, 테이블 단위 repo).
- `import_boundary_test.go` 구조. 간선을 지울 때는 같은 diff에서 표를 고친다.
- G가 승인되기 전까지 ADR-057의 pipeline 보존 결정.
