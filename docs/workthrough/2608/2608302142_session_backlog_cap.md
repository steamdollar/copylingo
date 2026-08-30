# Scheduled Session backlog cap 3

## 배경

ADR-042의 기존 정책은 Quiz/Study 미완료 세션이 하나라도 있으면 새 scheduled session을 차단했다. 무한 누적은 방지했지만 한 세션을 놓친 뒤 새 content가 전혀 노출되지 않는 문제가 있었다.

## 결정

- Quiz/Study를 합산한 사용자별 `pending`/`in_progress` 상한을 `schedule.max_unfinished_sessions` 설정으로 관리하고 기본값을 3으로 두었다.
- 미완료 수가 cap 미만이면 현재 cron 종류의 새 세션을 생성·발송한다.
- cap 이상이면 ADR-042의 우선순위로 기존 세션 하나만 재알림한다.
- 설정값은 1~3만 허용하며 수동 생성·자동 expiry와 DB schema는 변경하지 않았다.

## 변경 파일

- `internal/repository/session_repo.go`, `internal/repository/session_repo_test.go`
- `internal/service/session_query.go`, `internal/service/session_query_test.go`
- `internal/scheduler/scheduler.go`, `internal/scheduler/session_reminder_test.go`
- `internal/config/config.go`, `internal/config/config_test.go`, `config.yaml`
- `docs/adr/ADR_from_41_to_60.md`, `STATUS.md`

## 검증

- `go test ./internal/repository ./internal/service ./internal/config ./internal/scheduler`: 통과
- `make test`: 통과
- `git diff --check`: 통과
- `make restart-app`: 통과, `http://localhost:8080/health` ready 확인
