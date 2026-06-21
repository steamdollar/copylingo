# Quiz 후보를 학습 완료 Material로 제한

## 배경

Study session으로 먼저 material을 학습한 뒤, Quiz session에서는 사용자가 아직 학습하지 않은 material의 문제가 나오지 않아야 한다.

## 변경 사항

- `internal/service/session_builder.go`
  - Quiz session 생성 시 `userID`를 SRS review 조회와 신규 question 조회에 전달하도록 변경.
- `internal/service/srs.go`
  - `GetDueReviews` 계약에 `userID`를 추가해 사용자별 학습 material 기준 필터링이 가능하도록 변경.
- `internal/repository/question_repo.go`
  - `GetNewQuestions`, `GetDueReviews`에서 `questions.material_id`와 `user_material_progress`를 조인.
  - `ump.user_id = $1 AND ump.times_studied > 0` 조건으로 학습 완료된 material의 question만 반환.
- 테스트 mock 및 repository query assertion 보강.

## 성능 판단

- 기존 `user_material_progress` PK가 `(user_id, material_id)`라 사용자별 progress 조회는 index-friendly 하다.
- `questions.material_id`에도 partial index가 있어 material 기반 join 비용은 크지 않다.
- 별도 cache나 denormalized table은 현재 필요하지 않다.

## 로컬 데이터 반영

- `go run ./cmd/admin/reset_learning_data -yes`
  - `users` 보존, learning data reset.
- `docker compose exec -T redis redis-cli -n 0 FLUSHDB`
  - reset 전 active session cache 제거.
- `go run ./cmd/ja/seeder`
  - materials 708개, questions 2122개 생성.
- `go run ./cmd/admin/build_study_sessions`
  - user 1명에 대해 pending Study session 1개 생성.

## 검증

- `go test ./internal/service ./internal/repository ./internal/bot ./cmd/ja/seeder`
- `make test`
- DB count 확인:
  - users: 1
  - materials: 708
  - questions: 2122
  - user_material_progress: 0
  - sessions: 1
  - session_materials: 8
- reset 직후 studied quiz candidates: 0
- `make restart-app`
  - `http://localhost:8080/health` ready 확인.

## 주의 사항

reset 직후에는 아직 완료한 material이 없으므로 Quiz session 후보가 0개다. Telegram에서 먼저 pending Study session을 완료하면, 해당 material에 연결된 questions만 이후 Quiz 후보로 들어온다.
