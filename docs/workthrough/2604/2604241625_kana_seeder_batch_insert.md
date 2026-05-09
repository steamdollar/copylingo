# 작업 기록: kana seeder batch insert + transaction 적용

## 작업 목적

`cmd/kana_seeder`가 문항을 하나씩 insert하던 구조를 `transaction + batch insert`로 변경해 성능과 원자성을 함께 개선했습니다.

## 변경 사항

- `QuestionRepository.CreateBatch(...)`를 추가했습니다.
- batch insert는 하나의 transaction 안에서 수행되며, 실패 시 전체 rollback 됩니다.
- multi-row `INSERT ... VALUES (...), (...), ... RETURNING id` 형태로 한 번에 여러 문항을 생성합니다.
- `cmd/kana_seeder/main.go`는 per-question insert 대신 먼저 `[]*model.Question`을 구성한 뒤 한 번에 저장하도록 변경했습니다.
- 기존 `seedQuestion`, `seedHandwritingQuestion`은 insert 함수에서 builder 함수로 역할을 바꿨습니다.
- batch query placeholder 생성이 깨지지 않도록 repository 단위 테스트를 추가했습니다.

## Verification

- `go test ./...` 통과
- `internal/repository/question_repo_test.go` 추가

## 메모

- 이 변경은 runtime hot path가 아니라 seeder 경로에만 적용했습니다.
- `parallel insert`는 transaction atomicity와 코드 복잡도를 고려해 도입하지 않았습니다.
