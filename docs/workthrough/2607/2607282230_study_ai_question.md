# Study Session AI 질문

- **날짜**: 2026-07-28
- **유형**: Case A(ADR-037) → Case B
- **관련 ADR**: [ADR-037](../../adr/ADR_from_21_to_40.md#adr-037-study-material-질문은-기존-one-shot-llm-flow를-사용자-소유-context로-확장한다)

## 배경

Quiz 결과에는 문제 맥락을 넣는 `🤖 질문` 버튼이 있었지만, Study card에는 같은 학습 중 질문 경로가 없었다. 사용자가 Material 제목과 내용을 다시 적지 않고 현재 card를 기준으로 질문할 수 있어야 했다.

## 변경

| 파일 | 변경 |
|---|---|
| `internal/config/constants.go` | `study:{session_id}:ask:{material_order}` callback 및 `study:` pending token 규약 추가 |
| `internal/bot/study_flow.go` | owner-only 질문 버튼, callback에서 Study session ownership·Material membership 검증, 10분 one-shot token 저장 |
| `internal/bot/llm_question.go` | 사용자 소유 active Study session에서 해당 card를 다시 로드해 category·title·표시 내용을 LLM prompt에 결합 |
| `internal/bot/*_test.go` | owner gate, callback token, Material/ownership 검증, context rendering 테스트 추가 |

## 결정

- 기존 Quiz의 Redis one-shot flow와 `LLMService.AnswerLearningQuestion`을 그대로 재사용했다. 별도 LLM interface나 database schema는 만들지 않았다.
- callback 수신과 사용자 text 수신 사이에 session 상태가 바뀔 수 있으므로, context 로드 시에도 `GetOwned`와 Material order 검증을 반복한다.
- 완료된 Study session 또는 만료된 Redis state에서는 Material context를 넣지 않고 기존 plain LLM 질문으로 fallback한다.

## 검증

- `go test ./internal/bot` 통과.
- `make test` 전체 회귀 테스트 통과.
- `make restart-app` 후 `http://localhost:8080/health`가 `{"status":"healthy"}`를 반환했다.
