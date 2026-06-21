# JA Seeder 통합

## 배경

기존 JA seed flow는 `material_seeder`, `kana_seeder`, `vocab_seeder` 세 command로 분리되어 있었다.
Task 4의 실제 요구는 한 번의 seed 실행에서 Material을 먼저 만들고, 그 Material ID에 맞춰 기존 Kana/Vocabulary Question 유형을 생성하는 것이다.

## 변경

- `cmd/ja/seeder`를 단일 JA seed command로 통합했다.
- 실행 순서:
  1. `ja.BuildAllMaterials()`로 Kana/Vocabulary Material upsert.
  2. `material_key`로 Kana/Vocabulary Material ID 조회.
  3. 기존 Kana Question 유형 생성.
  4. 기존 Vocabulary Question 유형 생성.
  5. `question_key` 기준 Question upsert.
- `questions.question_key`를 stable seed key로 추가했다.
- JA Seeder는 deterministic generation을 사용한다.
  - Kana map iteration은 정렬한다.
  - Kana question의 random type/options는 입력값 기반 deterministic seed를 사용한다.
  - Vocabulary option shuffle은 고정 seed를 사용한다.
- 기존 command 제거:
  - `cmd/ja/material_seeder`
  - `cmd/ja/kana_seeder`
  - `cmd/ja/vocab_seeder`
- 현재 실행 명령 문서 갱신:
  - `go run ./cmd/ja/seeder`

## 검증

```bash
go test ./cmd/ja ./cmd/ja/seeder -v
make test
PGPASSWORD=copylingo make migrate
go run ./cmd/admin/reset_learning_data -yes
docker compose exec -T redis redis-cli -n 0 FLUSHDB
go run ./cmd/ja/seeder
go run ./cmd/ja/seeder
```

테스트 통과.
Seeder 2회 연속 실행 후 `questions=2122`, `distinct question_key=2122`, duplicate `0건` 확인.

## 주의

`cmd/ja/seeder`는 Material과 seed Question 모두 upsert한다.
다만 로컬 학습 상태까지 완전히 초기화하려면 기존처럼 `cmd/admin/reset_learning_data -yes`를 먼저 실행한다.
