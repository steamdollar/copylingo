# 일별 JSONL Structured Logging 도입

## 작업 범위

Application Log를 Standard Library `log/slog` 기반 JSONL 형식으로 전환했다.
stdout 출력을 유지하면서 `logs/copylingo-YYYY-MM-DD.jsonl` 파일에도 동시에 기록한다.

## 결정 사항

- 외부 logger Dependency를 추가하지 않고 `log/slog`를 사용한다.
- 파일명과 JSON timestamp는 기본 `Asia/Seoul` 기준이다.
- 일별 파일은 30일 보관하며 규약에 맞는 만료 파일만 자동 삭제한다.
- 파일 sink 장애 시 stderr 경고 후 stdout-only로 degrade한다.
- HTTP, Telegram Update, Scheduler job 진입점에서 `interaction_id`를 생성하고 `context.Context`로 전파한다.
- 숫자 식별자와 latency는 기록하지만 token, Telegram `init_data`, 사용자 답안 원문, stroke 좌표는 기록하지 않는다.
- 기존 `log.Printf`는 `slog.SetDefault()` 호환 경로로 수용하고 주요 경계부터 점진적으로 구조화한다.

## 변경 파일

- `internal/observability/`
  - Thread-safe 일별 writer, 30일 cleanup, JSON logger 초기화, Context attribute Handler, correlation ID 생성기를 추가했다.
- `cmd/server/`, `internal/bot/`, `internal/scheduler/`
  - HTTP Middleware, Telegram Update wrapper, Scheduler job wrapper를 추가했다.
  - HTTP panic recovery를 Context-aware structured log로 교체했다.
- `internal/miniapp/`, `internal/service/`, `internal/external/`
  - 손글씨 제출 Handler → Service → Grader → LLM latency 로그를 동일 correlation 경로로 연결했다.
  - 제출 후 Background Telegram 버튼 정리는 부모 correlation을 보존하되 요청 취소 신호에서는 분리했다.
- `internal/config/config.go`, `config.yaml`, `.env.example`
  - logging 경로, Level, 보관 기간, timezone 설정을 추가했다.
- `.gitignore`, `docker-compose.yml`
  - `logs/`를 Git에서 제외하고 Docker host volume으로 보존한다.
- `README.md`, `docs/ARCHITECTURE.md`, `docs/adr/ADR_from_21_to_40.md`
  - 운영 방법, 구조도, ADR-021을 기록했다.

## 검증 결과

```bash
git diff --check
make test
make build
go test -race ./internal/observability ./internal/bot ./internal/miniapp
docker compose config
```

모두 통과했다. `docker compose config`에서 `./logs:/app/logs` mount도 확인했다.

## 기존 작업 보존

작업 시작 전에 존재하던 손글씨 yoon tolerance prompt와 관련 테스트 및 ADR 변경은 유지했다.
