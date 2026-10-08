# Quiz 답안 제출 소유자 검사 + callback 정수 파싱 오류 처리

- 날짜: 2026-10-05
- 계기: ADR-059 완료 후 구조 검토(Case 0)에서 발견한 정합성 문제 중 U1 단위. 후속 단위는 [docs/todos/structure_review_followups.md](../../todos/structure_review_followups.md).

## 변경

| 파일 | 내용 |
|---|---|
| `internal/service/session_quiz_submit.go` | `SubmitQuizOption`·`SubmitQuizWordOrder`에 `userID`, `QuizTextAnswer.UserID` 추가. `loadQuizForAnswer`가 `Session.UserID != userID`면 `ErrQuizActiveSessionUserMismatch` 반환 (`!= 0` 생략 분기 없음) |
| `internal/bot/session_flow.go` | `quizSession` 계약 시그니처 갱신. `fmt.Sscanf`(에러 무시) 2곳을 `strconv.Atoi` + 실패 시 무시로 교체 |
| `internal/bot/session_answer.go` | `cb.From.ID`/`msg.From.ID` 전달. 소유자 불일치는 WARN `telegram.answer.owner_mismatch` 로그 후 무응답 |
| `internal/bot/word_order.go` | `cb.From.ID` 전달 |
| 테스트 | `TestSessionServiceSubmitQuizRejectsOtherUser`(선택지·어순·주관식), `TestHandleAnswerCallback_NonNumericOptionIndexIsIgnored` 추가, 기존 fixture의 세션 소유자를 callback 발신자와 일치 |

## 판단

- **문제**: 선택지 callback `q:{sessionID}:{qid}:{idx}`는 위조 가능하다. 기존에는 제출 경로에 소유자 검사가 없어 다른 사용자 세션에 답이 기록될 수 있었다(IDOR). 세션 시작·어순 draft·손글씨는 이미 검사하고 있었다.
- **검사 위치**: 세 제출 경로가 공유하는 `loadQuizForAnswer` 한 곳. bot 어순 경로의 사전 검사는 draft 관리용이라 유지한다.
- **파싱**: `Sscanf` 실패 시 index 0으로 처리되어 0번 선택지가 제출됐다. 잘못된 callback은 무시한다.
- **보류**: 구조 검토의 SRS 간격 반올림 불일치(Go 절사 vs SQL `ROUND`)는 SRS 단일화 결정(U4)에 포함한다. callback 이중 응답은 실기기 확인 후 처리한다.

## 검증

- `make test` 통과
- `make restart-app` 후 `/health` healthy
