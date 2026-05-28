# 손글씨 채점 Feedback 정책 정리

## 배경

손글씨 채점 결과가 정답인 경우에도 LLM 이 칭찬성 feedback 을 생성하거나, Mini App 이 explanation 을 덧붙여 결과 문구가 불필요하게 길어질 수 있었다.

목표는 정답/오답 문구의 책임을 Mini App 이 안정적으로 맡고, LLM 은 오답일 때 필요한 correction note 만 선택적으로 제공하게 하는 것이다.

## 변경 사항

- `internal/external/llm.go`
  - `GradeHandwriting` system prompt 에 feedback policy 를 추가했다.
  - 정답이면 `feedback` 은 empty string.
  - 오답이면 expected text 는 반복하지 않고, 필요한 경우 짧은 한국어 correction note 만 반환.
  - 칭찬/격려/filler 문구를 금지.
  - strict JSON schema 의 `feedback` description 에 같은 정책을 반영.
- `internal/external/llm_test.go`
  - 손글씨 prompt 에 feedback policy 가 포함되는지 검증.
  - schema 의 `feedback.description` 에 correct-empty 정책이 있는지 검증.
- `web/miniapp/handwriting/app.js`
  - 정답 결과는 항상 `정답입니다.`만 표시.
  - 오답 결과는 `오답입니다. 정답은 {correct_answer} 입니다.`를 기본으로 표시하고, LLM feedback 이 있을 때만 뒤에 붙인다.
  - 더 이상 `payload.explanation`을 result status fallback 으로 붙이지 않는다.
- `web/miniapp/handwriting/index.html`
  - `app.js` cache-busting query 를 갱신했다.

## 검증 결과

```bash
go test ./internal/external
make test
```

모두 통과.

## 미검증

- JS 전용 unit test harness 는 현재 없으므로 추가하지 않았다.
- 실제 Telegram Mini App 에서 최종 UX 는 수동 e2e 로 확인해야 한다.
