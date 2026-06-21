# Quiz 후보 Study Material 우선순위 + Level Fallback

## 배경

직전 구현은 `user_material_progress.times_studied > 0`인 material의 question만 Quiz 후보로 허용했다. 이 정책은 reset 직후나 Study 진행량이 적은 사용자에게 Quiz session이 생성되지 않는 문제가 있었다.

## 변경 사항

- `internal/repository/question_repo.go`
  - 신규 question 조회를 `JOIN` 기반 strict filter에서 `LEFT JOIN` 기반 priority sort로 변경.
  - `ORDER BY CASE WHEN ump.material_id IS NOT NULL THEN 0 ELSE 1 END`로 학습한 material의 question을 먼저 배치.
  - 부족한 경우 같은 `language`, `proficiency_level`, `category` 조건의 전체 question pool에서 fallback.
  - due review 조회도 동일하게 학습 material 기반 review를 우선 배치하되, 부족하면 전체 due review로 보충.
- `internal/repository/question_repo_test.go`
  - query assertion을 strict filter 검증에서 priority fallback 검증으로 변경.

## 정책

1. Study 완료 material question을 우선 출제한다.
2. 목표 session size를 채우기 부족하면 user level에 맞는 question을 랜덤 fallback으로 채운다.
3. category 제한은 유지한다. 예: vocabulary reserved slot은 vocabulary fallback pool에서만 채운다.

## 검증

- `go test ./internal/repository ./internal/service ./internal/bot`
- `make test`
- DB 확인:
  - 현재 `studied_progress = 0`
  - fallback query 후보 `2122`
- `make restart-app`
  - `http://localhost:8080/health` ready 확인.
