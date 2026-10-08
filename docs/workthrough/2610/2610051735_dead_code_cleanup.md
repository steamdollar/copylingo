# 구조 검토 U2: 죽은 코드 삭제

- 날짜: 2026-10-05
- 계기: [구조 검토 후속 계획](../../todos/structure_review_followups.md) U2. 직전 단위는 [U1](2610051431_quiz_answer_owner_check.md).
- 동작 변화: 없음 (실행 경로가 없는 코드만 제거)

## 삭제 내역

| 대상 | 파일 |
|---|---|
| 호출자 0인 repository 메서드 10개 (`SessionRepository.GetByID`·`Complete`·`GetTodaySessions`, `SessionQuestionRepository.GetBySession`·`RecordAnswer`·`GetWrongAnswers`, `QuestionRepository.CreateBatch`·`GetByID`, `UserRepository.Update`, `ContentRepository.GetArticles`)와 고아가 된 `buildQuestionBatchInsertQuery` | `internal/repository/*` |
| 쓰기만 하고 읽지 않는 Redis 키 `session:%d:question_start` (`RecordQuestionStart`, `QuestionTimingStore`, `SessionFlowDeps.Timing`, 호출부 2곳, 조립부) | `internal/redisstore/interactions.go`, `internal/bot/{interactions,session_flow,session_question}.go`, `cmd/server/server.go` |
| 생산자 측 interface `TTSClient`·`AudioStore` | `internal/external/{tts_client,audio_store}.go` |
| method set이 같은 unexported interface 5개 → exported(`QuizActiveSessionRepo`·`StudyActiveSessionRepo`·`QuizGradingLLM`·`MaterialRepo`·`SessionQuestionRepo`)로 통일 | `internal/service/{quiz_active_session,study_active_session,grader,study_session,session_builder}.go` |
| `quizActiveSessionScheduler` → `*srsService` | `internal/service/quiz_active_session.go` |
| grader의 테스트 전용 `GradeAnswer`·`GradeHandwriting`·`questionFromQuizActiveSession`, `graderQuizActiveSession.Get` | `internal/service/grader.go` |
| `model.UserMaterialProgress` | `internal/model/material.go` |
| scheduler의 콘텐츠 수집 연결 `Orchestrator`·`collectContent`, `cmd/server`의 `initPipeline` | `internal/scheduler/scheduler.go`, `cmd/server/server.go` |
| 테스트 전용 잔재 `mockStatsRepo`, `mockSRS.processAnswerFn`, 제거된 메서드의 mock | `internal/{bot,service}/*_test.go` |

## 판단

- **콘텐츠 수집**: 사용자 결정에 따라 scheduler·server 연결만 제거했다. `pipeline` 패키지·`ContentService`·`content_repo`·NHK client는 ADR-057 보존 결정대로 남긴다. ADR-057에 보강 1줄을 추가했다. import allowlist의 scheduler 항목은 `{"model", "observability"}`로 좁혔다.
- **grader 테스트**: 채점 동작을 검증하던 6개는 `GradeAnswerWithQuestion`·`GradeHandwritingWithQuestion` 호출로 전환했다. `TestGradeAnswer_AlreadyAnswered`만 삭제했다. 이 테스트의 대상은 제거된 조회 경로였고, 같은 경로는 `quiz_active_session_test.go`의 중복 거부 테스트 2개가 검증한다.
- **diff 크기**: 편집한 파일에 goparams가 적용되어 줄 수가 늘어 보인다. 추가된 줄에 HEAD에 없던 식별자·문자열이 없음을 확인했다(포맷 변경과 삭제만 있음).

## 검증

- `make test` 통과, `go vet ./...` 출력 없음
- `golangci-lint run --enable-only unused ./...` → 0 issues (작업 전 2건)
- `make restart-app` 후 `/health` healthy
