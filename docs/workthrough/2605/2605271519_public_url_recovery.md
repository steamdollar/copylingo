# 서버 재시작 후 Mini App public URL stale 복구 안정화

## 배경

Cloudflare Quick Tunnel 재시작 후 `.env`의 `COPYLINGO_SERVER_PUBLIC_BASE_URL`은 새 URL로 갱신됐지만, 서버 프로세스가 부모 shell/tmux에 남아 있던 stale `COPYLINGO_SERVER_PUBLIC_BASE_URL`을 우선 읽어 예전 WebApp URL을 Telegram 버튼에 넣고 있었다.

실제 확인:

- `.env`: `https://burner-gossip-invalid-confident.trycloudflare.com`
- 서버가 보낸 WebApp 버튼: `https://decisions-vat-lottery-however.trycloudflare.com/...`
- 프로세스 환경: stale `COPYLINGO_SERVER_PUBLIC_BASE_URL=https://decisions-vat-lottery-however.trycloudflare.com`

## 변경 파일

- `internal/config/config.go`
  - `viper.Reset()`을 추가해 `Load()` 호출 간 global state 오염을 방지했다.
  - 설정 key를 `BindEnv`로 명시해 `AutomaticEnv + Unmarshal` 환경변수 누락 가능성을 제거했다.
  - `COPYLINGO_SERVER_PUBLIC_BASE_URL`은 `.env` 값이 있으면 해당 값을 우선 적용하도록 했다. Quick Tunnel script가 `.env`를 갱신하므로, 이 key는 `.env`를 local dev SSOT로 취급한다.
- `internal/config/config_test.go`
  - stale inherited env보다 `.env`의 fresh public URL이 우선되는지 검증했다.
  - `.env`가 없으면 기존처럼 env public URL을 사용하는지 검증했다.
- `internal/bot/handler.go`
  - `ClearInlineKeyboard`가 빈 keyboard struct를 보내지 않고 `reply_markup`을 생략하도록 수정했다.
  - 기존 방식은 Telegram API에 `inline_keyboard:null`로 전달되어 400 에러가 발생했다.
- `internal/bot/handler_test.go`
  - `ClearInlineKeyboard`가 `ReplyMarkup == nil`인 `EditMessageReplyMarkupConfig`를 보내는지 검증했다.

## 검증

- `go test ./internal/config -v`
- `go test ./internal/bot -v`
- `make test`
- `make tmux-stop && make tmux`
- public tunnel health check:
  - `https://largest-plain-judgment-smith.trycloudflare.com/health` → `200 OK`

## 수동 복구 결과

서버 재시작 후 restart recovery sweep이 session `111`의 다음 미응답 손글씨 문제를 감지해 새 URL 메시지를 재전송했다.

- old message cleanup: `message_id=3809` inline keyboard 제거 성공
- new message: `message_id=3811`
- new WebApp URL: `https://largest-plain-judgment-smith.trycloudflare.com/miniapp/handwriting?...`

처음 실패했던 더 오래된 stale 메시지 `3807`도 Telegram API로 수동 cleanup 했다.

## 결정 사항

`COPYLINGO_SERVER_PUBLIC_BASE_URL`만 `.env` 우선순위를 갖는다. 다른 민감 정보(token/key/password)는 기존처럼 환경변수 주입을 유지한다. 이유는 Quick Tunnel URL은 `scripts/start_quick_tunnel.sh`가 `.env`에 쓰는 runtime-local 값이고, stale inherited env가 남으면 Mini App 버튼이 즉시 깨지기 때문이다.
