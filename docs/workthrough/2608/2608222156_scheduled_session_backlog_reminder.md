# Scheduled Session 미완료 backlog 재알림

## 배경

Morning/Evening Quiz와 정오/오후 Study scheduler는 기존 `pending` 또는 `in_progress` 세션을 확인하지 않고 매번 새 세션을 생성했다. 미완료 학습이 쌓이지 않도록 scheduled push가 기존 세션을 우선 재알림하도록 변경했다.

## 결정

- ADR-042에 정책을 기록했다.
- Quiz/Study 전체에서 `in_progress`를 우선하고, 같은 상태에서는 가장 오래된 세션을 선택한다.
- 미완료 세션이 있으면 새 session build를 생략하고 저장된 mode에 맞게 다시 발송한다.
- 수동 `/study`, `expired` 전환, 기존 backlog 일괄 정리는 변경하지 않는다.
- query와 build 사이의 multi-instance TOCTOU 방지는 별도 확장성 과제로 남긴다.

## 변경 파일

- `internal/repository/session_repo.go`, `internal/repository/session_repo_test.go`
- `internal/service/session_query.go`, `internal/service/session_query_test.go`, `internal/service/services.go`
- `internal/scheduler/scheduler.go`, `internal/scheduler/session_reminder_test.go`
- `docs/adr/ADR_from_41_to_60.md`, `STATUS.md`

## 검증

- `go test ./internal/repository ./internal/service ./internal/scheduler` — 통과
- `make test` — 통과
- `make restart-app` — 성공
- `http://localhost:8080/health` — ready 확인
