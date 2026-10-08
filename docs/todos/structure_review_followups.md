# 구조 검토 후속 작업 (U2~U4 + callback 이중 응답)

> 2026-10-05 ADR-059 완료 후 구조 검토(Case 0)의 결과. U1(Quiz 답안 소유자 검사·callback 파싱)은 완료 — [workthrough](../workthrough/2610/2610051431_quiz_answer_owner_check.md).
> 줄 번호는 커밋 54b7736 + U1 기준이다. 어긋나면 심볼로 다시 찾는다.

## 확정된 결정 (재논의 불필요)

- **하위 패키지 분리는 하지 않는다.** service·bot·나머지 패키지 검토 3건이 모두 같은 결론이다.
  - bot을 Flow별 패키지로 나누면 export해야 하는 것: unexported 식별자 약 30개, `botMessages` 필드 172개, `test_common_test.go`의 testutil 승격.
  - 그 대가로 컴파일러가 막아주는 Flow 간 간선은 `SessionFlow→StudyFlow` 1개뿐이다.
  - service 하위 패키지 후보는 `handwriting_render.go` 하나다(std만 import). 얻는 이득이 dev 도구 link 범위뿐이라 보류한다.
  - U3에서 이 판단을 비용 수치와 함께 ADR-059 §8.10으로 남긴다.
- **파일이 많아 보이는 원인은 이름이다.** 같은 개념에 필드명·타입명·파일명이 따로 붙어 있고, 테스트 파일이 소스와 대응하지 않는다.
- **진행 순서는 U2 → U3 → U4다.** 각 단위는 독립 커밋이고, `make test` 통과가 완료 조건이다.
  - 삭제(U2)를 rename(U3)보다 먼저 한다. 지울 파일을 옮기지 않기 위해서다.

## 0. callback 이중 응답 (사용자 실기기 확인 대기)

- **문제**: router가 모든 callback에 빈 `AnswerCallback`을 먼저 보낸다(`internal/bot/handler.go:438`). 그 뒤 `settings.go` 7곳과 `material_preference.go` 5곳이 다시 Answer/Alert를 보낸다. 에러는 무시된다.
  - Telegram이 callback당 한 번만 응답을 받는다면 이 경고·토스트는 표시되지 않는다.
- **확인 방법**: 설정 화면에서 alert가 떠야 하는 동작을 실행한다. 예: 시간 슬롯 검증 실패, 자료 설정 오류.
- **수정 방향 (확인 후)**: 응답 책임을 한쪽에 둔다.
  - 권장안: settings·material preference prefix의 callback은 router가 사전 응답하지 않는다.
  - 이때 해당 handler의 모든 경로가 응답하는지 확인한다.

## U2. 죽은 코드 삭제 (동작 변화 없음) — ✅ 완료 2026-10-05

> 완료: [workthrough](../workthrough/2610/2610051735_dead_code_cleanup.md). 수집 경로는 scheduler·server 연결만 제거하고 pipeline 코드는 보존했다(사용자 결정). 아래 표는 기록용이다.

| 대상 | 위치 | 근거 |
|---|---|---|
| 호출자 0인 repository 메서드 10개 | `session_repo.go` `GetByID`(:157)·`Complete`(:340)·`GetTodaySessions`(:377), `session_question_repo.go` :38·:49·:67, `question_repo.go` `CreateBatch`(:30)·`GetByID`(:63), `user_repo.go` `Update`(:47), `content_repo.go` `GetArticles`(:34) | grep 0건(테스트 포함). `Complete`는 상태 가드와 flush가 없어 위험하다. bot/service 테스트 mock의 같은 메서드도 함께 지운다. |
| Redis `session:%d:question_start` | `redisstore/interactions.go:18,89`, `bot/interactions.go:56` `QuestionTimingStore`, `SessionFlowDeps.Timing`, 쓰는 곳 `bot/session_question.go:120`·`bot/session_flow.go:721` | 읽는 코드가 없다(Go·JS). Initial commit부터 그렇다. |
| 생산자 측 interface | `external/tts_client.go:26` `TTSClient`, `external/audio_store.go:24` `AudioStore` | 참조 0건. §8.8에서 `LLMClient`를 지운 것과 같은 이유다. |
| method set이 같은 interface 5쌍 | `service/session.go:95`=`quiz_active_session.go:36`, `:107`=`study_active_session.go:36`, `:131`=`grader.go:15`, `:119`=`study_session.go:154`, `:87`=`session_builder.go:55` | Tier2 필드 타입을 exported 쪽으로 통일한다. subset interface(`questionFetcher` 등)는 mock이 쓰므로 유지한다. |
| `quizActiveSessionScheduler` | `service/quiz_active_session.go:47` | test double이 없다. 모두 `newSRSService(nil)`을 쓴다. |
| grader의 테스트 전용 메서드 | `service/grader.go:61` `GradeAnswer`, `:134` `GradeHandwriting`, `:266` `questionFromQuizActiveSession` | `grader_test.go`에서만 호출한다. 같은 경로를 `session_quiz_submit_test.go`·`quiz_active_session_test.go`가 검증한다. |
| `model.UserMaterialProgress` | `model/material.go:34` | 참조 0건 |
| scheduler 콘텐츠 수집 경로 | `scheduler/scheduler.go:25,106` `Orchestrator`, `:191` `collectContent`, `cmd/server/server.go:93` `initPipeline` | `golangci-lint unused` 2건. allowlist의 scheduler→pipeline 간선이 이것 때문에 남아 있다. **ADR-057(수집 비활성·코드 보존)과 충돌하므로 먼저 사용자 확인이 필요하다.** 최소안은 scheduler에서만 제거하고 `initPipeline`을 `cmd/admin/collect_content` 진입점으로 옮기는 것이다. |

## U3. rename·이동 (동작 변화 없음, `git mv` 위주)

**service**: `session_quiz_*` / `session_study_*` prefix를 Tier2에도 적용한다(ADR-059 §8.6 규칙의 확장). 용어는 `progress`로 통일한다.

| 현재 파일 (타입, `SessionService` 필드) | 새 파일 (타입·필드) |
|---|---|
| `session_builder.go` (`sessionBuilderService`, `selection`) | `session_quiz_builder.go` (`quizBuilder`) — Quiz 전용임을 호출처로 확인 |
| `quiz_active_session.go` (`quizActiveSessionService`, `quizProgress`) | `session_quiz_progress.go` (`quizProgress`) |
| `grader.go` + `errors.go` (`graderService`) | `session_quiz_grader.go` (`quizGrader`) |
| `srs.go` | `session_quiz_srs.go` — Quiz 전용 |
| `handwriting_render.go` | `session_quiz_handwriting_render.go` |
| `study_session.go` (`studySessionService`, `studyBuilder`) | `session_study_builder.go` (`studyBuilder`) |
| `study_active_session.go` (`studyActiveSessionService`, `studyProgress`) | `session_study_progress.go` (`studyProgress`) |
| `level_policy.go` | `session_level_policy.go` |

테스트 파일도 같은 이름으로 바꾸고, 다음을 정리한다.
- `mockSRS`를 `grader_test.go:52`에서 builder 테스트 쪽으로 옮긴다.
- `TestSessionServiceBuildForSlot`(`session_study_test.go:183`)을 `session_dispatch_test.go`로 옮긴다.
- `session_quiz_test.go`를 start/complete로 나눈다.
- `session_builder_material_preference_test.go`는 실제로 maintenance cap을 테스트하므로 그에 맞게 이름을 바꾼다.
- 공유 fake를 `session_fakes_test.go`로 모은다.
- `session_builder_test.go`(1951줄)는 level scope 테스트를 level_policy 테스트와 합치고, 나머지는 mix/caps로 나눈다.

문서는 `docs/ARCHITECTURE.md:39-41`과 `docs/todos/user_selectable_session_mix_presets.md:6,54`를 갱신한다.

**bot**

| 이동 | 이유 |
|---|---|
| `handler.go:21-41` (`sanitizeTelegramHTML`, `BotAPI`) → `telegram_client.go` | 쓰는 곳이 telegram_client뿐이다 |
| `handler.go:81-507` → `router.go`, `:509-988` → `router_commands.go` | 라우팅과 명령 handler를 분리한다 |
| `session_flow.go:167-435` → `session_menu.go`, `SessionFlow.StartStudy` → `ResumeFromMenu` | 새 세션을 만들지 않는 메뉴 재개 handler라 이름과 doc이 틀렸다 |
| `study_flow.go:695-998` → `study_render.go` | `llm_question.go:368`도 쓰는 공유 renderer다 |
| `word_order.go` → `session_word_order.go`, `restart_recovery.go` → `session_restart_recovery.go`, `settings.go` → `settings_flow.go` | Flow prefix |
| `webapp.go` → `session_question.go`에 병합 | 유일한 소비처다 |
| `interactions.go` → `session_flow.go`에 병합 후 unexport | `SessionFlowDeps`에서만 쓴다 |
| truncate·stripHTML(`session_helpers.go:95-124`), `mainMenuKeyboard`, `escapeHTML`(`study_flow.go:1000`), `sanitizeTelegramHTML` → `telegram_text.go` | 흩어진 텍스트 helper를 모은다. `llm_question.go:308`의 `html.EscapeString` 직접 호출도 통일한다 |
| `material_preference.go` 유지 | 3개 Flow가 공유하는 helper다. 헤더 주석만 추가한다 |
| 패키지 내부 전용인데 export된 11개 unexport | `StartStudy`·`StartReview`·`HandleSessionCallback`·`HandleAnswerCallback`·`HandleTextInput`·`StudyFlow.HandleCallback`·interactions interface 4개·`BotAPI` |

**bot 테스트 재배치**
- `coverage_boost_test.go` 해체(:122-432 → session_flow, :433-605 → session_question, :606-739 → session_answer, :740-909 → router_commands, :912 → telegram_client).
- `handler_dispatch_test.go`의 LLM 테스트(:386-720)는 llm_question으로, /study 테스트는 router_commands로 옮긴다.
- `session_flow_extended_test.go`, `util_recovery_test.go`, `handler_test.go`, `sanitize_test.go`, `logging_test.go`는 대응하는 소스 파일 이름으로 옮긴다.
- `session_flow_test.go:13-131`은 `callback.*`만 호출하므로 `internal/callback/callback_test.go`로 옮긴다.
- 중복을 삭제한다: `coverage_boost:509`=`session_question_test:656`, `session_flow_test:132`⊂`session_question_test:144`, `session_question_test:17`(pass-through wrapper `session_question.go:579-587`과 함께). helper `ptr`=`strPtr`, `settingsMockUserRepo`=`mockUserRepo`.
- assert 없는 coverage용 테스트(`session_flow_extended:451`, `coverage_boost:262,:401`)는 assert를 넣거나 삭제한다. 작업 메모 주석(`session_flow_extended_test.go:486-490`)은 지운다.
- `TestStartReview_NoneDue`(:290)와 `_Actual`(:396)은 이름이 반대로 붙어 있다.

**나머지**
- `cmd/server`
  - `server.go` → `bot.go`: HTTP 서버가 없는데 이름이 server다.
  - `router.go`와 `logging.go`를 `http.go`로 합친다.
  - 하드코딩 경로(`logging.go:59`, `miniapp/handler.go:90`)를 상수로 바꾼다.
- redisstore
  - `sessions.go`와 `session_store.go`를 합친다.
  - `model/session_store.go`(sentinel 2개)는 `model/session.go`로 흡수한다.
- callback 주석: `callback/callback.go:10`의 "also created by the Mini App"은 stale이다. 현재 importer는 bot뿐이다.
- stale 주석
  - `service/srs.go:30`의 "graderService가 srsService에 의존"은 사실과 다르다.
  - `session_builder.go:62`와 `quiz_active_session.go:67`의 "learning sessions"는 Quiz를 가리킨다.
  - `bot/telegram_client.go:12`는 `*Bot` 경유를 설명한다.
  - `bot/handler.go:81`의 "every dependency is required"는 `:732`의 nil 분기와 모순된다.
- ADR-059:326의 `Repositories` 사용처는 "cmd/server"가 아니라 "cmd/*"다.

## U4. 책임 이동 (일부 동작 변화, 항목별 Case A 선결)

| 항목 | 위치 | 제안 | 선결 |
|---|---|---|---|
| Study Tier1이 전달만 함 | `service/session_study.go`(6개 중 5개 forward), 흐름 조정은 `study_active_session.go:115,248-288`. DB fallback이 `:83-98`과 `:166-190`에 중복 | Start/Complete 흐름 조정을 Tier1로 올리고 Tier2를 load/save/delete/mark/flush로 축소해 Quiz와 같은 모양으로 맞춘다. `studyActiveSessionStarter` 제거 | ADR-059 §8.2 기준 위반이라 결정 불필요 |
| grader 채점과 기록 분리 | 정상 기록은 `grader.go:121,206`, AI 실패 시 대체 기록은 `session_quiz_submit.go:312`. "현재 문항/이미 답함" 검증 4곳 | grader는 (question, answer) → (correct, feedback)만 계산하고, `RecordAnswer`는 Tier1에서 한 번만 호출 | 없음 |
| Quiz 소유자 검사 4가지 방식 | `session_quiz_start.go:116`(`!= 0` 생략, 도달 불가: `sessions.user_id` NOT NULL), `quiz_active_session.go:245`, `session_quiz_complete.go:32`, `session_quiz_handwriting.go:95` | `quizProgress.loadOwned` 하나로 통일. 제출 경로는 U1에서 처리 | 없음 |
| SRS 정책 2벌·반올림 불일치 | Quiz는 Go `srs.go:83-93`(`int()` 절사), Study는 SQL `study_active_session_repo.go:420-437`(`ROUND`). interval 15·ease 2.5에서 37일 vs 38일 | 단일 구현 여부와 반올림 규칙(원본 SM-2는 올림)을 결정 | **ADR 필요** |
| `*_active_session_repo` 이름이 실제 역할을 숨김 | `repository/quiz_active_session_repo.go:16`, `study_active_session_repo.go:15`: Postgres 스냅샷 적재와 4테이블 완료 커밋(자체 tx, ADR-061의 유일한 예외). 에러 prefix가 rename 전 이름 `ActiveSessionRepository.FlushActiveSession` 그대로 | `QuizCompletionRepository`·`LoadQuizSnapshot`·`CommitQuizCompletion` 식 rename, prefix 수정, 수동 tx를 `WithinTx`로 | rename 범위 확인 |
| scheduler의 backlog 정책 | `scheduler/dispatcher.go:13` `maxUnfinishedSessions=3`, `:177` 호출 순서 조정 | `SessionService.SessionForSlot(...) (session, isReminder, err)`로 흡수. mode→pusher 라우팅은 scheduler에 둔다 | ADR-053 갱신 |
| LLM 질문 준비 코드 3벌 | `bot/session_flow.go:605-659`, `study_flow.go:586-673`, `llm_question.go:87-139` | `llm_question.go`의 free function `armLLMQuestion`으로 합침. `studyInputStore` 제거 | 없음 |
| `buildSession` 350줄 | `service/session_builder.go:145-495` | review / due / new relay / persist를 private method로 분리 | 없음 |
| Redis 상태 손상 시 처리 | `ErrSessionStoreCorrupt`를 쓰는 곳 0. redisstore가 키를 지운 뒤 service는 NotFound만 재구성 | Corrupt도 DB에서 재구성 | 동작 변화, 테스트 동반 |

## 검증

- 단위마다 `make test`
- 런타임에 영향이 있으면 `make restart-app` 후 `/health`
- U3는 `git diff -M --stat`에서 rename이 인식되는지 확인

## 손대지 말 것

- import allowlist(`internal/import_boundary_test.go`)의 구조. 간선 제거는 같은 diff에서 표를 수정한다.
- ADR-057 콘텐츠 수집 비활성 결정. U2의 scheduler 경로 삭제는 사용자 확인 후에만 한다.
