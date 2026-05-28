# 손글씨 LLM 채점 튜닝

## 배경

`kana_handwriting` 채점 호출의 응답 시간 편차와 응답 format 흔들림을 줄이기 위해 strict JSON schema 적용 이후 generation bound를 추가했다.
이후 실제 사용 테스트에서 latency 대부분이 `render`/DB가 아니라 LLM multimodal 호출 구간에 있고, 작은 화면 손글씨 입력에서 false negative가 발생하는 것을 확인했다.

따라서 이번 작업은 두 방향을 하나의 튜닝으로 정리한다.

1. output generation을 제한해 format 안정성과 불필요한 출력 비용을 줄인다.
2. prompt rubric을 beginner-practice 기준으로 조정해, 엄격한 시험식 채점보다 작은 화면 손글씨 UX와 false negative 감소를 우선한다.

## 변경 파일

- `internal/external/llm.go`
  - 손글씨 채점용 `ChatCompletionRequest` 생성을 `buildHandwritingChatCompletionRequest` helper로 분리했다.
  - `MaxCompletionTokens`를 80으로 제한했다.
  - `Temperature`를 0.01로 설정했다.
  - 기존 `ResponseFormat: json_schema`, image detail `low` 설정은 유지했다.
  - 손글씨 system prompt를 긴 rule list에서 `Task / Accept when / Reject when / Feedback policy` rubric으로 재구성했다.
  - `not open-ended OCR`, `full expected string`, identity-changing marks 기준을 명시했다.
  - 작은 화면 손글씨의 false negative를 줄이기 위해 `Expected Text`가 plausible 하면 accept 쪽으로 기울이는 기준을 추가했다.
  - 비슷한 kana 간 애매한 경우에는 명확한 mismatch가 없으면 reject하지 않도록 조정했다.
  - 정답 feedback은 empty string 유지, 오답 feedback은 선택적 짧은 한국어 한 문장으로 제한했다.
- `internal/external/llm_test.go`
  - 손글씨 채점 요청에 generation bound와 image detail 설정이 포함되는지 검증하는 테스트를 추가했다.
  - prompt가 binary verification boundary와 accept/reject decision rubric을 포함하는지 검증하는 테스트를 추가했다.

## 결정 사항

### Generation bound

- `Temperature`는 0이 아니라 0.01로 설정했다.
  - 현재 사용 중인 `go-openai`의 `Temperature` 필드는 `omitempty`이므로 0을 넣으면 JSON 요청에서 빠진다.
  - 실제 전송을 보장하면서 deterministic에 가깝게 만들기 위해 0.01을 사용했다.
- `MaxCompletionTokens`는 80으로 시작한다.
  - 응답 schema가 `is_correct`와 짧은 `feedback`뿐이므로 충분한 상한이다.
  - 너무 낮게 잡으면 JSON truncation 위험이 있어 40 이하로 줄이지 않았다.
- `ReasoningEffort`는 적용하지 않는다.
  - `low` 적용 후 Gemini 응답이 JSON이 아닌 `Here`로 잘려 반환되어 parsing failure가 발생했다.
  - `MaxCompletionTokens`가 reasoning token까지 포함되는 구조에서 visible JSON 출력 안정성을 해칠 수 있어 rollback했다.

### Prompt rubric

- 초보자 손글씨 관용성은 유지한다.
  - wobble, uneven stroke width, size, spacing, tilt, mobile drawing 문제는 reject 사유가 아니다.
- 정확도보다 UX 관용성을 우선한다.
  - LLM이 stroke-by-stroke forensic analysis를 하지 않고 빠른 beginner-practice judgment를 하도록 지시한다.
  - `Expected Text`가 plausible 한 경우 false negative보다 false positive를 더 감수한다.
- 글자 identity를 바꾸는 요소는 명시적으로 reject 기준에 둔다.
  - dakuten, handakuten, small kana, sokuon, chouon 이 명확히 없거나 명확히 틀린 경우는 오답 처리한다.
  - rough/faint mark는 작은 화면 입력 특성상 plausible 하면 accept 한다.
- 속도 개선 효과는 prompt만으로 보장하지 않는다.
  - provider queue, vision input processing, network latency가 dominant하면 latency 개선은 제한적일 수 있다.

## 관련 ADR

- `docs/ADR.md`의 ADR-016: 손글씨 가나 채점은 False Negative 최소화와 빠른 판정을 우선

## 검증 결과

```bash
go test ./internal/external
make test
```

모두 통과.
