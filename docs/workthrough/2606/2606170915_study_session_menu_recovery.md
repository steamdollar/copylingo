# Study Session 메뉴 진입 및 stale session 복구

## 배경

learning data reset 이후 기존 Telegram 메시지의 session callback이 삭제된 DB row를 가리켰고, 새 Study session은 DB에 pending 상태로 있었지만 `menu:study` 경로가 quiz session만 찾고 있었다.

## 확인 결과

- `sessions`에는 `id=1`, `mode=study`, `status=pending`, `total_questions=8`만 존재.
- 08:00 morning quiz push는 `user_material_progress=0`이라 `No questions available`로 skip됨. 이는 학습한 material만 quiz 후보로 제한한 정책상 정상.
- 09:07에 누른 이전 Telegram session은 `session_id=9`였고, reset 이후 해당 row가 없어 `sql: no rows in result set`으로 실패.

## 변경 사항

- `internal/bot/session_flow.go`
  - `menu:study`에서 pending/in-progress `mode=study` session을 먼저 처리.
  - pending Study session이 있으면 `study:{session_id}:start` 버튼을 노출.
  - in-progress Study session이 있으면 Study flow로 resume.
- `internal/bot/session_helpers.go`
  - `firstStudySession` helper 추가.
- `internal/repository/session_repo.go`
  - `Start`, `Complete`에서 `RowsAffected == 0`이면 `session not found` 에러 반환.
  - 삭제된/stale session callback이 조용히 진행되지 않도록 방어.
- `internal/bot/session_flow_extended_test.go`
  - pending Study session이 `study:10:start` callback으로 표시되는 regression test 추가.

## 운영 조치

- 앱 재시작: `make restart-app`
- pending Study session `1`을 Telegram으로 직접 재발송.
  - Telegram response `ok=true`
  - message_id: `6357`

## 검증

- `go test ./internal/bot ./internal/repository ./internal/service`
- `make test`
- `curl -fsS http://localhost:8080/health`
- DB 확인:
  - `sessions`: `id=1`, `mode=study`, `type=study`, `status=pending`, `total_questions=8`

## 참고

이전 Telegram 메시지는 reset 전 session id를 들고 있으므로 재사용할 수 없다. 새로 발송한 Study session 메시지 또는 `메뉴 -> 학습하기` 경로를 사용해야 한다.
