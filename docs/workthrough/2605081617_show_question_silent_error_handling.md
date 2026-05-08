# showQuestion silent error 처리

> TODO 출처: `docs/todos/show_question_silent_error_handling.md` (작업 완료 후 삭제됨)

## 배경

`internal/bot/session_flow.go`의 `showQuestion`에서 `SessionBuilder.GetQuestion(ctx, sq.QuestionID)`가 실패하면 로그/사용자 안내 없이 그대로 `return`했다. `session_questions.question_id`가 깨졌거나 `questions` row가 누락된 경우, 사용자는 버튼을 눌러도 봇이 죽은 것처럼 보이고 운영자는 어떤 세션/문항에서 실패했는지 추적할 수 없었다.

같은 함수 내 `GetSessionQuestions` 실패 처리(line 200-209)에 이미 검증된 패턴이 있어 이를 그대로 차용했다.

## 변경 내용

**파일**: `internal/bot/session_flow.go`

`GetQuestion` 호출 직후 silent return 분기(line 227-231)를 다음과 같이 교체:

```go
sq := sqs[questionIdx]
question, err := sf.bot.services.SessionBuilder.GetQuestion(ctx, sq.QuestionID)
if err != nil {
    log.Printf("Error getting question for session %d question_idx=%d question_id=%d: %v",
        sessionID, questionIdx, sq.QuestionID, err)
    if editMessageID != nil {
        sf.bot.EditMessage(chatID, *editMessageID, "❌ 문제를 불러오지 못했습니다. 잠시 후 다시 시도해 주세요.", nil)
    } else {
        sf.bot.SendMessage(chatID, "❌ 문제를 불러오지 못했습니다. 잠시 후 다시 시도해 주세요.")
    }
    return
}
```

기존 `// TODO: silent error fix` 주석 제거. import 추가/제거 없음 (`log`, `tgbotapi` 등 이미 import됨).

### 일관성 결정 사항

- 로그 prefix는 line 201의 `"Error getting session questions for session %d: %v"`와 톤을 맞춤 — bot handler 경계 계층은 함수명 prefix를 박지 않고 작업 맥락만 적는다(AGENTS.md 에러 처리/로깅 정책 §5.1).
- 사용자 메시지 문자열은 line 202와 완전 동일 ("❌ 문제를 불러오지 못했습니다. 잠시 후 다시 시도해 주세요.").

## 검증

- `go build ./...` — 통과
- `make test` — 모든 패키지 통과 (캐시 적중, 추가/변경된 테스트 없음)

### 테스트 작성 미수행 사유

`bot.SessionFlow`는 mock 인프라가 없고 (`internal/bot`은 `[no test files]`), 이 변경은 동일 함수 내 line 200-209의 검증된 패턴을 단순 복제하는 수준이라 회귀 위험이 낮다고 판단해 수동 검증으로 대체. 향후 `service` 레이어 인터페이스 도입 후 bot 핸들러 테스트도 함께 도입하는 것이 자연스러움 (`docs/todos/service_layer_interfaces_and_tests.md` 참고).

### 수동 검증 절차 (운영 시)

- DB에서 임의의 `session_questions.question_id`를 존재하지 않는 ID로 변경 → 봇 세션 진행
- 기대 동작:
  1. 서버 로그: `Error getting question for session <id> question_idx=<n> question_id=<x>: ...`
  2. 사용자 메시지: `"❌ 문제를 불러오지 못했습니다. 잠시 후 다시 시도해 주세요."`
- 정상 세션(객관식/주관식/손글씨)이 기존과 동일하게 동작하는지 회귀 확인
