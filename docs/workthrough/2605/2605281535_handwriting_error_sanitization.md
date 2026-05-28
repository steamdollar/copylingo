# 손글씨 LLM 오류 사용자 노출 차단

## 배경

손글씨 Mini App 채점 중 LLM provider 500 또는 provider-specific error 가 발생하면, 기존 handler 는 `err.Error()`를 그대로 HTTP response body 에 넣었다.

이 경우 사용자에게 내부 wrapping message, provider body, 운영 세부 정보가 노출될 수 있으므로 HTTP boundary 에서 public message 로 변환하도록 정리했다.

## 변경 사항

- `internal/miniapp/handler.go`
  - `SubmitHandwriting` error branch 에서 `err.Error()`를 그대로 반환하지 않도록 변경.
  - `handwritingPublicError` helper 를 추가해 domain error 별 public status/message 를 매핑.
  - 원본 error 는 서버 로그에 남기고, response body 에는 sanitized message 만 반환.
  - handler testability 를 위해 필요한 service dependency 를 handler-local interface 로 분리.
- `internal/miniapp/handler_test.go`
  - LLM/provider raw error 가 response body 로 유출되지 않는지 검증.
  - unauthorized, already answered, invalid question, AI config missing mapping 검증.

## 결정 사항

- HTTP boundary 에서 public error message 를 만든다.
- 하위 계층의 error wrapping 은 유지한다.
- unknown grading failure 는 현재 `503 Service Unavailable` 로 매핑한다.
- non-LLM operational error taxonomy 를 더 세밀하게 나누는 작업은 이번 범위에서 완료하지 않았다. 필요 시 service/external sentinel error 를 추가하는 별도 리팩터로 처리한다.

## 검증 결과

```bash
go test ./internal/miniapp
make test
```

모두 통과.
