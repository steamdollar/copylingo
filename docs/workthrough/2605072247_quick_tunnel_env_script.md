# 작업 기록: Cloudflare Quick Tunnel URL 자동 반영 스크립트 추가

## 작업 목적

Cloudflare Quick Tunnel을 사용할 때 매번 출력 URL을 복사해 `.env`에 직접 반영하는 과정을 줄이기 위해 자동화 스크립트를 추가했습니다.

## 반영 내용

- `scripts/start_quick_tunnel.sh`를 추가했습니다.
  - `cloudflared tunnel --url http://localhost:8080`을 실행합니다.
  - 출력 로그에서 `https://*.trycloudflare.com` URL을 추출합니다.
  - `.env`의 `COPYLINGO_SERVER_PUBLIC_BASE_URL` 한 줄만 추가 또는 교체합니다.
  - 기존 Telegram token, LLM API key 등 다른 `.env` 값은 유지합니다.
- `Makefile`에 `make tunnel` target을 추가했습니다.
- `README.md`와 `docs/HANDWRITING_MINIAPP_INGRESS.md`의 Quick Tunnel 실행 절차를 `make tunnel` 기준으로 갱신했습니다.

## 사용 방법

```bash
make tunnel
```

URL이 갱신되면 CopyLingo 서버를 재시작해야 합니다.

```bash
make run
```

## 주의사항

- Quick Tunnel URL은 임시 URL이므로 tunnel 프로세스가 새로 시작되면 바뀔 수 있습니다.
- 이 스크립트는 BotFather 설정을 자동화하지 않습니다.
- 현재 CopyLingo는 메시지의 Inline Keyboard `web_app` URL을 서버가 직접 생성하므로, 우선 `.env` 반영 자동화만 수행합니다.
