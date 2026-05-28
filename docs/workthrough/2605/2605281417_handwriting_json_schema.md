# 손글씨 채점 응답 포맷 Strict JSON Schema 적용

## 배경

`kana_handwriting` 채점에서 LLM 응답 format 이 일정하지 않을 가능성이 있어, 기존 `json_object` 응답 형식을 더 강한 `json_schema` 계약으로 변경했다.

## 변경 사항

- `internal/external/llm.go`
  - `GradeHandwriting`의 `ResponseFormat`을 `json_object`에서 `json_schema`로 변경.
  - `buildHandwritingResponseFormat` helper를 추가해 `is_correct`, `feedback` 두 필드만 허용하도록 `strict=true` schema를 정의.
- `internal/external/llm_test.go`
  - 손글씨 채점 response format 이 `json_schema`, `strict=true`, `additionalProperties=false`, 필수 필드 2개로 구성되는지 검증하는 단위 테스트 추가.

## 결정 사항

- 이번 변경은 prompt 문구 자체를 수정하지 않고, API 요청의 출력 계약만 강화한다.
- provider unsupported 상황에서 자동 fallback retry 는 넣지 않았다. 장애/timeout/ratelimit 상황까지 재시도하면 latency tail 이 더 커질 수 있기 때문이다.
- Gemini OpenAI compatible endpoint 의 live 지원 여부는 로컬 테스트만으로 검증하지 않았다. 실제 호출에서 `response_format=json_schema` 거부가 발생하면 fallback 또는 `json_object` rollback 을 별도 판단한다.

## 검증 결과

```bash
go test ./internal/external
make test
```

모두 통과.
