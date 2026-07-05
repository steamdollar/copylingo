# TODO: Cloudflare Tunnel(cloudflared/trycloudflare) Korea-block 노출 대응

> **상태**: Case A 선결(대응 방식 미결) — 실행 전 사용자와 접근법 확정 필요.
> 청해(ADR-031/032) 조사 중 발견된 out-of-scope 리스크를 기록만 한 문서다. 지금 건드리지 않는다.

## 배경 / 목적

- 손글씨 Mini App은 로컬 서버(`localhost:8080`)를 **Cloudflare Quick Tunnel**로 외부 노출해 Telegram Mini App에서 연다. 유저 브라우저가 `*.trycloudflare.com` URL로 접속하므로 **user-facing Cloudflare 의존**이다.
- 2026 한국은 Cloudflare 관련 차단이 진행 중이다:
  - **2026-05**: 방통위 협조로 Cloudflare가 **CDN 레이어에서 특정 사이트를 한국 IP에 차단**(협조형, 지정 사이트 한정).
  - **2023**: 한국 ISP가 **R2 도메인(`r2.cloudflarestorage.com` 등)을 DNS poisoning으로 wholesale 차단**한 전례(일방형). → 한국이 특정 Cloudflare 서비스 도메인을 골라 차단할 능력·의사가 있음을 보여줌.
- 현재 `trycloudflare.com`은 어느 메커니즘의 표적도 아니라 **도달 정상**(2026-07-01 개발 머신 probe 확인). 그러나 poison 목록이 넓어지면 손글씨 Mini App이 **통째로 중단**될 수 있는 latent 리스크다.

## 현재 노출 지점 (변경 대상)

- [scripts/start_quick_tunnel.sh](../../scripts/start_quick_tunnel.sh) — `cloudflared tunnel --url http://localhost:8080` 실행, `*.trycloudflare.com` URL 파싱.
- [Makefile](../../Makefile) — tunnel 기동/재시작 타깃, `COPYLINGO_SERVER_PUBLIC_BASE_URL` 반영.
- `.env`의 `COPYLINGO_SERVER_PUBLIC_BASE_URL` — Mini App public base URL(현재 trycloudflare 도메인).
- 관련 문서: 손글씨 Mini App ingress ADR(`ADR_from_01_to_20.md` 인근), README Mini App/Tunnel 설정 절.

## 후보 접근법 (Case A에서 택1 — 미결)

| 안 | 내용 | 트레이드오프 |
|---|---|---|
| A. 자체 도메인 + named tunnel | 소유 도메인에 Cloudflare **named tunnel**(quick 아님) 연결 | 여전히 Cloudflare 경유 → 한국이 CF를 넓게 막으면 동일 노출. 안정 URL·운영성은 개선 |
| B. 비-CF ingress로 전환 | ngrok/tailscale funnel/자체 리버스프록시(예: Caddy+VPS) 등 Cloudflare 밖 경로 | CF-Korea 리스크 원천 제거. 신규 의존/셋업·무료 tier 제약 검토 필요 |
| C. accept + monitor | 현행 유지 + 도달성 헬스체크/알림만 추가 | 최소 작업. 차단 시 즉시 중단 감수(단일 유저라 감내 가능할 수도) |

## 아직 정해지지 않은 것 (Case A 결정 포인트)

1. 위 A/B/C 중 어느 방향인가. (포트폴리오 §4 관점 + 운영 부담 + 8GB 제약 고려)
2. B라면 어떤 대체 ingress인가(무료 tier·안정성·Telegram Mini App https 요건 충족 여부).
3. 우선순위: 청해(진행 중) 완료 후인가, 그 전 선제 대응인가.

## 검증 방법 (실행 시)

- 전환 후 Telegram Mini App(손글씨)에서 실제 로드·손글씨 채점 왕복 E2E 확인.
- `make test` + 재시작 타깃(Makefile 헤더 매니페스트 참조) 후 `http://localhost:8080/health`.
- 한국 네트워크에서 신규 ingress URL 도달성 probe(dig/curl).

## off-limits / 메모

- 청해 기능(ADR-031/032)과 독립. 청해 진행을 막지 않는다.
- R2는 이 프로젝트에서 미채택(ADR-032)이므로 R2 차단 자체는 본 과제 범위 밖. 여기 대상은 **Mini App tunnel ingress**뿐.
