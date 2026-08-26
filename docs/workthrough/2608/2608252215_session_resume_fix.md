# Scheduled Session 재진입 진행상태 복구

## 배경

미완료 세션 재알림의 `시작하기` callback이 Quiz/Study working set을 매번 DB에서 다시 만들었다. 완료 전 답변과 학습 여부는 Redis에만 있으므로 부분 진행 상태가 초기화됐고, Quiz는 계산된 다음 미답변 위치 대신 첫 문제를 다시 표시해 `이미 답변한 문제입니다.`가 노출될 수 있었다.

## 결정 및 변경

- ADR-042를 보정해 진행 중 세션의 SSOT를 Redis working set으로 명시했다.
- Quiz 재진입은 Redis를 우선하고 miss일 때만 DB에서 복구하며, 첫 미답변 문제부터 표시한다.
- 최초 `pending` Quiz는 DB start 후 Redis 상태도 `in_progress`로 새로 맞춘다.
- Study 재진입도 Redis를 우선하고 첫 미학습 카드부터 표시한다.
- 이미 처리된 objective/text callback은 오류 메시지 대신 다음 미답변 문제 또는 결과 버튼으로 self-heal한다.
- session owner가 아닌 사용자의 Quiz 시작 callback을 거부한다.

## 변경 파일

- `internal/bot/session_flow.go`, `internal/bot/session_answer.go`
- `internal/bot/session_flow_extended_test.go`, `internal/bot/session_answer_test.go`
- `internal/service/study_active_session.go`, `internal/service/study_active_session_test.go`
- `docs/adr/ADR_from_41_to_60.md`, `STATUS.md`

## 검증

- `go test ./internal/bot ./internal/service` — 통과
- `make test` — 통과
- `git diff --check` — 통과
- `make restart-app` — 성공
- `http://localhost:8080/health` — ready 확인
