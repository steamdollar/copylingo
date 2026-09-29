# ADR-059 §8 B단계 구현 계획: SessionService Tier1 통합

- 상태: 계획 승인됨(2026-09-30), 구현 미착수
- 기준 문서: [ADR-059 §8](../adr/ADR-059_architecture_simplification.md#8-보강-2026-09-30-계층-규칙과-세분화된-실행-순서) — 특히 §8.6 세부 결정
- 보존 제약: [ADR-061·062·063](../adr/ADR_from_61_to_80.md)
- 직전 단계: [A단계 workthrough](../workthrough/2609/2609300018_bot_telegram_client_extraction.md)

## 1. 목표

bot·scheduler·miniapp이 service의 Tier1 타입만 참조하고 Tier2는 unexported로 둔다.

- Tier1: `SessionService`(Quiz+Study+SessionQuery) · `UserService` · `MaterialPreferenceService` · `AnalyzerService` · `TipService`(TipGenerator 흡수) · `LLMQuestionService` · `AudioService`
- Tier2(unexport 대상): `SessionBuilderService`, `QuizActiveSessionService`, `SRSService`, `GraderService`, `HandwritingService`, `StudySessionService`, `StudyActiveSessionService`, `LLMService`, `TipGenerator`
- `ContentService`는 cmd/server 파이프라인 전용이라 범위 밖(그대로 둠).

Tier1은 bot이 하던 조정 로직을 실제로 흡수해야 한다. 메서드 전달만 하는 층은 완료로 보지 않는다. 화면 렌더링용 읽기 전용 상태 조회는 Tier1 조회 메서드로 남는다(Get 호출 수 0이 목표가 아님).

## 2. Discovery 결과 (2026-09-30, commit f0ebe79 기준)

Tier2 의존 계층:

```
Handwriting ─► Grader ─► QuizActiveSession ─► SRS
                 └─► LLM, userRepo(streak)
SessionBuilder ─► SRS
StudySession, StudyActiveSession, SessionQuery : leaf
```

- `SessionQuery`는 service 내부에서 쓰는 곳이 없는 leaf. 호출자는 scheduler(`CountUnfinishedBatch` scheduler.go:201, `GetOldestUnfinished` dispatcher.go:260). `CountUnfinished`는 테스트 외 호출자 없음.
- `Audio`: bot `session_question.go` `sendListeningAudio`에서 `GetClip`·`CacheFileID`(Telegram file_id 캐시, 실패 시 재업로드), scheduler에서 `TopUpAudio`. leaf.
- service 테스트 19개 파일은 전부 `package service`(내부 테스트) → Tier2 unexport 후 타입명 치환만으로 재사용 가능.
- bot 테스트 17개 파일(그중 13개가 `&Bot{services: &service.Services{...}}` 조립)은 repo fake로 `service.NewXxxService`를 만든다 → 같은 fake를 `SessionDeps`에 넣도록 바꾼다.
- `SRS.GetDueCount` 직접 호출 현재 위치: `bot/handler.go:582`(메인 메뉴), `bot/session_flow.go:256`(StartReview).
- `repository.ListInProgress`는 `mode = 'quiz'`만 조회. `GetSessionsByStatus`는 모드 무관.

## 3. SessionService 설계

생성자: `NewSessionService(SessionDeps{...})` — 필드는 소비자 정의 인터페이스.

| 필드 | 현재 출처 |
|---|---|
| QuestionRepo | repos.Question (SessionBuilder·SRS) |
| SessionRepo | repos.Session (SessionBuilder·StudySession·StudyActive·SessionQuery) |
| SessionQuestionRepo | repos.SessionQuestion |
| QuizActiveSessionRepo | repos.QuizActiveSession |
| StudyActiveSessionRepo | repos.StudyActiveSession |
| MaterialRepo | repos.Material |
| UserRepo | repos.User (streak 갱신) |
| Stores | `SessionStores{Quiz, Study}` — 합치지 않음(ADR-059 §8.6) |
| LLM | 채점용 LLM (Tier2 `llm`) |
| DB | `*sqlx.DB` (ADR-061 `WithinTx`) |

Tier2(selection·quizProgress·srs·grader·studyBuilder·studyProgress)는 생성자 안에서 만든다. Tier2 간 호출은 같은 패키지 concrete 호출로 바꿔도 된다(§8.4).

파일: `session.go`(타입·Deps·생성자), `session_query.go`, `session_dispatch.go`, `session_quiz_start.go`, `session_quiz_submit.go`, `session_quiz_handwriting.go`, `session_quiz_complete.go`, `session_study.go`. 이름은 구현 시 조정 가능.

### 3.1 공개 메서드 초안 (정확한 시그니처는 구현 시 확정)

모드 공통:

| 메서드 | 호출자 | 흡수 |
|---|---|---|
| `BuildForSlot(user, slot)` | scheduler dispatcher | dispatcher.go:141-152의 slot → Study/Quiz 생성 분기 |
| `CountUnfinishedBatch` | scheduler | (조회) |
| `OldestUnfinished` | scheduler 재알림 | (조회, 모든 모드) |
| `ListByStatus(userID, status)` | bot getPendingSessions/getInProgressSessions | (조회, 모든 모드) |

Quiz:

| 메서드 | 호출자 | 흡수 |
|---|---|---|
| `BuildMorningQuiz`/`BuildEveningQuiz` | bot `/test`, BuildForSlot 내부 | 이동 |
| `BuildReviewQuiz(user)` | bot StartReview | due count → 0이면 `ErrNoDueReviews` → 15개 상한 → 생성. **due count 오류는 기존대로 0건 취급**, WARN 로그 추가 |
| `DueReviewCount` | bot 메인 메뉴 | (조회) |
| `StartQuiz(sessionID, userID)` | bot | session_flow.go:549-625: Get → 소유자 확인(`UserID != 0` 조건 유지) → wasPending → DB Start → pending이면 CreateFromDB. bot이 실패 유형별로 다르게 반응하므로(조회/소유자/DB 시작 실패는 로그만, 재적재 실패는 `sessionStatePrepareFailed` 전송) 구분 가능한 sentinel error 반환 |
| `ShowQuizQuestion(sessionID, idx)` | bot showQuestion | Get + SetCurrentIndex(현재 Get 2회) 결합. idx ≥ len이면 set 없이 상태 반환 |
| `SubmitQuizOption(sessionID, questionID, optionIdx)` | bot processAnswer | 현재 문제·중복 확인, 옵션 해석, 채점 |
| `SubmitQuizText(sessionID, questionIndex, text, …)` | bot HandleTextInput | index→questionID, FillBlank 소문자화, 채점 |
| (공통 내부) | | AI 채점 불가 시 `RecordAnswer(false)` 대체 기록 + 결과에 "채점 불가" 플래그, AlreadyAnswered 매핑. 주관식 채점 전 typing 표시 필요 → 결과/훅 방식은 구현 시 결정(동작 보존) |
| `SubmitHandwriting(req)` | miniapp | 기존 HandwritingService 흐름 그대로 |
| `CompleteQuiz(sessionID, userID)` | bot finishSession | word-order 문제 ID 수집(소유자 일치 시) → Flush → streak → Delete. 결과 + word-order ID 반환, bot이 draft 삭제 |
| `QuizProgress(sessionID)` | bot 렌더·LLM 문맥·자료 설정·word order·재시작 복구, miniapp 갱신 | (읽기 전용) |
| `ListInProgressQuizzes` | bot 재시작 복구 | (조회) |

Study:

| 메서드 | 호출자 | 흡수 |
|---|---|---|
| `BuildStudy(user, profile, limit)` | scheduler, bot `/study` | 이동(ADR-061 트랜잭션 유지) |
| `StartStudy` | bot | 이동 |
| `MarkStudied` | bot nextMaterial | 이동 |
| `FinishStudy(sessionID, userID, lastOrder)` | bot finishSession | MarkStudied → Complete. 실패 단계별 sentinel(`saveCompletionFailed`/`completeFailed` 문구 구분 유지) |
| `CompleteStudy` | bot showMaterial 자동 완료 | 이동 |
| `StudyProgress(sessionID, userID)` | bot prev·LLM 문맥·자료 설정·ask | (읽기, 기존 LoadOwnedStudySessionState) |

### 3.2 손글씨 경로

- `HandwritingService` → `SessionService.SubmitHandwriting`. `HandwritingSubmitRequest/Result`, `ErrHandwriting*`, `Stroke`, `StrokeRenderer`, `NewDefaultPNGStrokeRenderer`는 공개 유지(miniapp 오류 매핑, cmd/dev 사용).
- 유형별 실패 정책 유지: 손글씨는 AI 채점 불가 시 오류 반환(대체 기록 없음), 주관식은 오답 대체 기록.
- miniapp 비동기 갱신은 `QuizProgress`로 재조회(현재 동작 유지). 결과에 index를 실어 재조회를 없애는 안은 "이미 다음 문제로 넘어간 경우" 동작이 바뀌므로 채택하지 않음. Telegram 버튼 구성 이동은 C단계.

### 3.3 기타 Tier1

- `TipService`: `TopUpBucket` 흡수, TipGenerator unexport. LLM client가 generator를 지원하지 않을 때 nil 허용 동작 유지(scheduler의 nil 가드 동작 보존).
- `LLMQuestionService.Answer(user, username, prompt, question)`: LLM 답변 → 팁 후보 저장(`sourceModel`은 생성자에서 값으로). 문맥 프롬프트 조립·사용자 조회·권한 확인은 bot 유지.
- `LLMService`는 Tier2 `llm`으로 unexport, SessionService(채점)와 LLMQuestionService가 공유.
- `service.Services` 필드: `Session, User, MaterialPreference, Analyzer, Tip, LLMQuestion, Audio, Content`. 묶음 삭제는 D단계.

## 4. 커밋 순서

각 커밋: 수정 Go 파일에 gofmt + goparams([CONVENTIONS](../CONVENTIONS.md#agent-go-formatting)) → `make test` 통과. 로컬 커밋만(push 별도 승인).

0. `style`: 수정 대상 중 goparams 미적용 파일의 포맷 변경만 분리(A단계 방식).
1. SessionService 뼈대 + Quiz 생성·시작·완료·복습·조회·모드 공통 조회. bot·scheduler·restart_recovery 호출 전환. SessionQuery 흡수.
2. Quiz 제출(선택지·텍스트) + 손글씨. bot answer 경로·miniapp 전환.
3. Study 흡수 + `BuildForSlot`.
4. TipGenerator → TipService, LLMQuestionService, `Services` 필드 정리.
5. Tier2 unexport(기계적 타입명 치환 — executor 위임 후보).
6. docs: workthrough, STATUS 단계 표시, ADR-059 상태 줄, 이 파일 삭제.

## 5. 테스트

- 흡수한 조정 로직마다 service 테스트 추가: StartQuiz(pending/in_progress/소유자 불일치/재적재 실패), 채점 불가 대체 기록, 중복 제출, 복습(due 0건/15 상한/count 오류), CompleteQuiz word-order ID, FinishStudy 단계별 실패, BuildForSlot 분기.
- 대체 기록을 검증하던 bot 테스트는 service 테스트로 이동. bot 테스트는 화면 분기만 검증.
- 기존 service 테스트는 타입명 치환으로 유지.

## 6. 보존할 정책 (변경 금지)

working set(Redis가 진행 중 SSOT, in_progress 상태를 DB로 덮어쓰지 않음), 트랜잭션(ADR-061), 멱등성(scheduler claim), batch 조회, worker/rate limit, 유형별 채점 실패 정책, Quiz 완료만 streak 갱신, due count 오류 0건 취급.

## 7. 범위 밖 / 미결

- Flow 분리·`*Bot` 역참조 제거: C단계.
- `Services` 묶음 삭제·외부 클라이언트 생성 이동: D단계.
- **미결(사용자 판단 대기)**: 텍스트·선택지 답안 경로(`bot/session_answer.go`)에는 소유자 확인이 없다(손글씨·시작·완료에는 있음). B단계에서는 추가하지 않는다. TODO 분리 여부는 사용자가 결정.
