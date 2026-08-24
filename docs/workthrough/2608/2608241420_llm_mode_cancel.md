# LLM mode 취소 버튼 추가

## 배경

`/llm`, Quiz `🤖 질문`, Study Material `🤖 질문`으로 one-shot LLM mode를 실수로 켠을 때, 기존에는 발견하기 어려운 `/exit` 명령만 취소 수단으로 제공됐다.

## 구현

- 모든 LLM pending mode 활성화 안내에 공통 `❌ 취소` inline keyboard를 추가했다.
- `llm:cancel` callback은 callback을 누른 사용자의 `UserLLMPendingRedisKey`만 삭제한다.
- 기존 `/exit`의 학습 입력 취소 동작과 DB·학습 session 상태는 변경하지 않았다.
- callback logging 분류에 `llm`을 추가했다.

## 변경 파일

- `internal/config/constants.go`
- `internal/bot/handler.go`
- `internal/bot/llm_question.go`
- `internal/bot/session_flow.go`
- `internal/bot/study_flow.go`
- `internal/bot/handler_dispatch_test.go`
- `internal/bot/llm_question_test.go`
- `STATUS.md`

## 검증

- `go test ./internal/bot` 통과
- `make test` 통과
- `git diff --check` 통과
- `make restart-app` 성공
- `curl -fsS http://localhost:8080/health` → `{"status":"healthy", ...}`

## 결정

새 architecture나 policy 변경는 없다. 기존 user-scoped Redis pending key를 그대로 사용하는 최소 UX 보완이므로 ADR은 추가하지 않았다.
