# 작업 기록: 손글씨 Mini App ingress/Cloudflare Tunnel 문서화

## 작업 목적

손글씨 문항의 submit/scoring을 위해 서버가 HTTP 요청을 받는 방식과, 개발 환경에서 Cloudflare Tunnel을 사용하는 이유 및 보안 주의사항을 ADR과 운영 문서에 기록했습니다.

## 반영 내용

- `docs/ADR.md`에 ADR-011을 추가했습니다.
  - 손글씨 제출을 Mini App HTTP `POST`로 받는 이유를 기록했습니다.
  - `init_data`, session ownership, question membership, duplicate answer 검증을 서버 책임으로 둔 결정을 명시했습니다.
  - 개발 ingress는 Cloudflare Tunnel을 우선 사용하고, 운영은 고정 도메인/reverse proxy/named tunnel을 권장하는 기준을 남겼습니다.
- `docs/HANDWRITING_MINIAPP_INGRESS.md`를 추가했습니다.
  - 사용자의 stroke 제출부터 LLM Binary Grading, DB 기록까지의 sequence diagram을 작성했습니다.
  - `cloudflared tunnel --url http://localhost:8080` 사용 절차를 정리했습니다.
  - tunnel 사용 시 보안상 주의할 점과 현재 코드에서 실제로 조치된 점을 분리해 기록했습니다.
  - 현재 `docker-compose.yml`의 DB/Redis host port publish는 운영 전 hardening 필요 항목으로 명시했습니다.
- `docs/ARCHITECTURE.md`에 손글씨 Mini App 제출 흐름을 추가했습니다.
- `README.md`의 Mini App 설정 섹션에서 상세 운영 문서로 연결되는 링크를 추가했습니다.

## 확인한 구현체

- `cmd/server/server.go`: Gin router에 Mini App route가 등록되고 HTTP 서버가 `:8080`에서 실행됨
- `internal/miniapp/handler.go`: `GET /miniapp/handwriting`, `POST /api/miniapp/handwriting/submit`
- `internal/service/handwriting.go`: session ownership, question membership, duplicate answer, question type 검증
- `internal/service/grader.go`: rendered handwriting image를 LLM Binary Grading으로 채점 후 결과 기록
- `internal/bot/session_flow.go`: `COPYLINGO_SERVER_PUBLIC_BASE_URL` 기반 Mini App URL 생성
- `docker-compose.yml`: app `8080`, PostgreSQL `5432`, Redis `6379` host port publish 상태

## 결정 사항

- 개발 단계에서는 OS/router port open보다 Cloudflare Tunnel을 우선 사용합니다.
- submit/scoring은 클라이언트가 아니라 서버에서 처리합니다.
- 운영 전에는 DB/Redis port exposure hardening과 submit endpoint rate limit 검토가 필요합니다.
