# Study Module Task 1: Material SSOT 및 Seed 추가

## 배경

기존 앱은 `questions`를 직접 Seed하고 출제하여 Quiz 이전의 Study 단계를 표현할 학습 단위가 없었다.
Study Module의 첫 단계로 `materials`를 Question과 분리된 SSOT로 추가했다.

## 변경 사항

- `migrations/001_init.sql`
  - `materials` 테이블 추가
  - `material_key UNIQUE`, nullable `content_id`, 공통 Metadata, `payload JSONB` 정의
- `internal/model/material.go`
  - `Material`, `MaterialCategory` 정의
- `internal/repository/material_repo.go`
  - Seeder용 `UpsertBatch` 추가
- `cmd/ja/material_seeder`
  - repo-owned curated N5 단어 목록에서 500개 Material 생성
  - Level을 Key에서 제외: `ja:vocab:word_024`
  - Question Seeder와 독립적으로 Vocabulary Material만 Idempotent Upsert
- `cmd/ja/kana_seeder`, `cmd/ja/vocab_seeder`
  - Material Upsert 책임을 제거하고 기존 Question Seed 책임만 유지

## 결정 사항

- `contents`는 외부 수집 원문으로 유지한다.
- `materials`는 Study 개념 SSOT로 사용한다.
- Question 연결, Material 단위 SRS, Study Session UI는 후속 Task로 보류한다.

## 검증

```bash
PGPASSWORD=copylingo make migrate
go run ./cmd/ja/material_seeder
make test
```

- Vocabulary Material 500건 Upsert
- 같은 Seeder 재실행 후에도 Vocabulary 500건 유지: Upsert Idempotency 확인
- 로컬 DB에는 분리 전 시험 실행으로 생성된 Kana Material 208건이 남아 있다. 삭제는 별도 승인 후 수행한다.
- `make test` 전체 통과
