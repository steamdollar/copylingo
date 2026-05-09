# 손글씨 Mini App tunnel 안정화 및 stale URL 복구 작업 기록

## 배경

손글씨 Mini App 버튼은 Telegram 메시지 발송 시점의 `COPYLINGO_SERVER_PUBLIC_BASE_URL`을 `web_app.url`에 고정한다.
Cloudflare Quick Tunnel 프로세스가 tmux 관리 범위 밖에서 종료되면 기존 `trycloudflare.com` host가 DNS resolve에 실패하고, 이미 발송된 버튼은 복구할 수 없다.

또한 `go-telegram-bot-api/v5.5.1`의 incoming `InlineKeyboardButton` 타입에는 `web_app` 필드가 없어 callback update에서 기존 메시지의 Mini App URL을 직접 읽을 수 없다.
따라서 새로 발송하는 손글씨 문제의 `next` callback data에 현재 public URL host fingerprint를 함께 넣고, 미제출 상태에서 fingerprint가 없거나 다르면 같은 문제를 새 URL로 재발송하도록 했다.

## 변경 파일

- `Makefile`
  - `make tmux`가 Tunnel, App, PostgreSQL log, Redis log 4개 pane을 생성하도록 변경했다.
  - Tunnel이 새 URL을 `.env`에 기록한 뒤 App이 시작되도록 `make tmux` recipe 내부에서만 짧게 대기한다.
  - `air`가 설치되지 않은 환경에서는 `go run ./cmd/server`로 fallback한다.
  - tmux 재시작/종료 시 기존 `cloudflared`, `go run ./cmd/server`, `:8080` listener를 inline command로 정리한다.
- `internal/bot/session_flow.go`
  - 손글씨 문제의 `next` callback data를 `q:{session_id}:next:{idx}:u:{fingerprint}` 형태로 생성한다.
  - 미제출 상태에서 token이 없거나 현재 `server.public_base_url` fingerprint와 다르면 stale URL로 판단한다.
  - stale이면 기존 메시지의 inline keyboard를 제거하고, 같은 문제를 fresh URL로 새 메시지에 재발송한다.
  - 기존 token 없는 메시지는 이전 형식으로 보고 stale 처리한다.
- `internal/bot/handler.go`
  - inline keyboard 제거용 `ClearInlineKeyboard` helper를 추가했다.
- `internal/bot/session_flow_test.go`
  - URL fingerprint, callback formatting, stale 판정 테스트를 추가했다.

추가 helper shell script는 만들지 않았다.
기존 `scripts/start_quick_tunnel.sh`만 그대로 사용한다.

## 검증

```bash
bash -n scripts/start_quick_tunnel.sh
make test
make tmux
curl http://localhost:8080/health
curl "$COPYLINGO_SERVER_PUBLIC_BASE_URL/miniapp/handwriting?session_id=1&question_id=1"
```

검증 결과:

- `make test` 전체 통과.
- `internal/bot`에 신규 단위 테스트 추가 및 통과.
- `make tmux` 후 pane 4개가 유지됨: Tunnel, App, PostgreSQL, Redis.
- `cloudflared tunnel --url http://localhost:8080` 프로세스는 1개만 유지됨.
- 로컬 `/health` 정상 응답.
- 새 tunnel URL 경유 Mini App HTML 응답 확인.

## 결정 사항

- 개발 환경에서는 Quick Tunnel을 tmux dashboard 생명주기에 포함한다.
- 필수 순서 보정은 "새 tunnel URL이 `.env`에 기록된 뒤 App 시작"에만 적용한다.
- helper script를 늘리지 않고, 단순 개발 운영 로직은 `Makefile` 안에서 관리한다.
- Quick Tunnel URL은 임시 URL이므로, 이미 발송된 Telegram `web_app.url`은 URL 변경 후 재사용할 수 없다.
- Telegram callback update에서 `web_app.url`을 직접 읽지 않는다. 현재 라이브러리 타입이 해당 필드를 보존하지 않기 때문이다.
- URL fingerprint는 보안 목적이 아니라 stale URL 판별용이므로 `fnv` 기반 8자리 hex token으로 충분하다.
- 기존 메시지처럼 fingerprint가 없는 callback은 stale로 간주해 한 번 재발송한다.
- 동일 host에서 tunnel 프로세스만 죽은 경우는 Bot callback만으로 판별하지 않는다. 이 경우까지 다루려면 public URL health check가 필요하지만 현재 범위에서는 제외한다.
