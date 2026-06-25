# Mini App → Bot coupling 제거: handwriting message ref parser 이동

## 배경

`docs/260621_architecture_package_refactor_review.md`의 Issue 2에서 `internal/miniapp`가 `internal/bot`을 import하는 coupling을 지적했다.

추가 검토 결과 `internal/callback` package가 이미 존재하고 bot/miniapp 양쪽에서 사용 중이므로, 별도 `internal/telegramref` package를 만들 필요 없이 `ParseHandwritingMessageRef`를 `internal/callback`으로 옮기는 것이 가장 작은 수정이다.

## 변경 파일

- `internal/callback/callback.go`
  - `ParseHandwritingMessageRef`를 shared parser로 추가.
- `internal/callback/callback_test.go`
  - parser 테스트를 callback package로 이동.
- `internal/bot/restart_recovery.go`
  - `callback.ParseHandwritingMessageRef` 호출로 변경.
- `internal/miniapp/handler.go`
  - `internal/bot` import 제거.
  - `callback.ParseHandwritingMessageRef` 호출로 변경.
- `internal/bot/util.go`
  - parser 이동 후 빈 util file이 되어 삭제.
- `internal/bot/util_recovery_test.go`, `internal/miniapp/handler_test.go`
  - parser 테스트 중복 제거.
- `docs/260621_architecture_package_refactor_review.md`
  - Issue 2의 실제 선택지와 완료 상태 반영.
- `STATUS.md`
  - 최근 완료 항목 추가.

## 결정 사항

- 새 `internal/telegramref` package는 만들지 않는다.
- handwriting message ref parser는 callback data/fingerprint helper와 같은 `internal/callback`에 둔다.
- `internal/miniapp`는 더 이상 `internal/bot`을 import하지 않는다.

## 검증

- `make test` 통과.
- `make restart-app` 통과.
- `curl -fsS http://localhost:8080/health` 응답:
  - `{"status":"healthy","time":"2026-06-22T15:39:58+09:00"}`
