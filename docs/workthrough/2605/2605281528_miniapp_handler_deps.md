# Mini App HandlerDeps 생성자 정리

## 배경

손글씨 submit error sanitization 테스트를 위해 `miniapp.Handler`가 필요한 dependency 를 interface 로 받도록 변경되어 있었다. 다만 `NewHandler(handwriting, tip, sessionBuilder, verifier, rdb, messenger, cfg)` 형태의 positional argument 가 7개라 테스트와 production 호출부 모두 인자 순서 실수에 취약했다.

## 변경 사항

- `internal/miniapp/handler.go`
  - `HandlerDeps` struct 추가.
  - `NewHandler(deps HandlerDeps)` 형태로 변경.
  - `RegisterRoutes`에서 named field 로 dependency 를 주입하도록 수정.
- `internal/miniapp/handler_test.go`
  - `NewHandler(nil, tipSvc, nil, ...)` 호출을 `NewHandler(HandlerDeps{Tip: tipSvc})`처럼 named field 방식으로 정리.

## 결정 사항

- Gemini 가 도입한 handler-local interface DI 는 유지한다.
- 기존 `service.Services` aggregate 로 되돌리지 않는다.
- 이번 변경은 constructor ergonomics 개선만 다루며, error taxonomy 보강은 별도 TODO 로 남긴다.

## 검증 결과

```bash
go test ./internal/miniapp
make test
```

모두 통과.
