# App 재시작 프로토콜 표준화

## 배경

Go 서버 코드, Mini App static asset, 설정을 변경한 뒤 매번 사용자가 "Makefile 보고 관련 인스턴스 재시작"을 별도로 요청해야 했다. 기존 `make tmux` dashboard는 Tunnel/App/DB/Redis를 함께 띄우지만, App만 재시작하는 표준 target이 없어서 pane 상태가 꼬이면 수동 tmux 조작이 필요했다.

## 변경 파일

- `Makefile`
  - `restart-app` target을 추가했다.
  - `copylingo` tmux session이 없으면 명확히 실패하고 `make tmux` 실행을 안내한다.
  - 기존 App pane을 찾아 제거한 뒤 새 `make dev` pane을 생성한다.
  - Tunnel, PostgreSQL, Redis pane은 건드리지 않는다.
  - 재시작 후 `http://localhost:8080/health` readiness를 최대 30초 확인한다.
  - `make tmux`가 새 pane에 `copylingo-tunnel`, `copylingo-app`, `copylingo-postgres`, `copylingo-redis` title을 붙이도록 정리했다.
- `AGENTS.md`
  - runtime에 반영되어야 하는 Go 서버, Mini App static asset, 설정 변경 후 `make restart-app`과 health check를 수행하도록 Case B 검증 규칙에 추가했다.
  - DB/Redis/Tunnel은 직접 관련 변경이 있을 때만 별도 재시작하도록 명시했다.

## 검증 결과

```bash
make -n restart-app
make restart-app
curl -fsS -o /dev/null -w '%{http_code}\n' http://localhost:8080/health
curl -fsS http://localhost:8080/miniapp/handwriting | rg 'app.js\?v=2606080100'
make test
```

결과: 모두 통과.

## 남은 주의점

- 기존에 떠 있던 tmux session은 `make tmux`로 생성된 시점이 오래되어 Tunnel/DB/Redis pane title은 아직 기존 shell title이다. `restart-app` 실행으로 App pane은 `copylingo-app` title이 붙었다.
- 다음에 `make tmux`로 dashboard를 새로 만들면 모든 pane title이 표준화된다.
