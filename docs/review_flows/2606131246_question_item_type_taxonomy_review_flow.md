# Question Item Type Taxonomy Review Flow

## Scope

- `questions.type`을 풀이/채점 방식으로 유지하면서 `questions.item_type`으로 JLPT 및 앱 내부 taxonomy를 분리한 변경을 리뷰한다.
- Schema, Go model, batch insert, 기존 seeder 생성 경로, DB backfill 기록, 테스트를 코드 흐름 순서대로 확인한다.

## Route Summary

| Category | Purpose |
|---|---|
| Decision & Backfill | `type`, `category`, `item_type`의 책임 분리와 기존 데이터 backfill 기준을 먼저 확인한다. |
| Schema & Model | DB column과 Go enum/struct가 같은 nullable taxonomy 계약을 표현하는지 확인한다. |
| Persistence Path | 신규 question batch insert가 `item_type`을 누락하지 않는지 확인한다. |
| Seeder Generation | 현재 kana/vocabulary seeders가 생성하는 6개 question 조합에 skill이 명확히 들어가는지 확인한다. |
| Tests | taxonomy enum, insert query, seeder skill 세팅을 회귀 테스트가 막는지 확인한다. |
| Runtime Notes | 로컬 DB 반영, 검증, 남은 nullable/analytics 리스크를 확인한다. |

## Review Order

### 1. Decision & Backfill

1. [docs/adr/ADR_from_21_to_40.md:173](../../docs/adr/ADR_from_21_to_40.md) — ADR-026에서 `questions.type`은 rendering/grading mode로 유지하고 `item_type`을 skill taxonomy로 분리한 결정을 확인한다.
2. [docs/workthrough/2606/2606131243_question_item_type_taxonomy.md:16](../../docs/workthrough/2606/2606131243_question_item_type_taxonomy.md) — 작업 기록의 결정 요약이 ADR과 같은 책임 분리를 말하는지 확인한다.
3. [docs/workthrough/2606/2606131243_question_item_type_taxonomy.md:49](../../docs/workthrough/2606/2606131243_question_item_type_taxonomy.md) — 로컬 DB `ALTER TABLE` 및 기존 row backfill SQL이 의미가 명확한 조합만 분류하는지 확인한다.
4. [docs/workthrough/2606/2606131243_question_item_type_taxonomy.md:67](../../docs/workthrough/2606/2606131243_question_item_type_taxonomy.md) — 기존 922개 row의 backfill 결과가 `type + category -> item_type` 매핑과 맞는지 확인한다.

### 2. Schema & Model

1. [migrations/001_init.sql:77](../../migrations/001_init.sql) — `questions` table에 `item_type VARCHAR(64)`가 nullable로 추가되어 legacy/unknown row를 허용하는지 확인한다.
2. [internal/model/question.go:8](../../internal/model/question.go) — 기존 `QuestionType`이 `multiple_choice`, `fill_blank`, `kana_handwriting` 등 풀이 방식 enum으로 유지되는지 확인한다.
3. [internal/model/question.go:30](../../internal/model/question.go) — `Skill`이 실제 측정 skill taxonomy로 추가되었는지 확인한다.
4. [internal/model/question.go:35](../../internal/model/question.go) — 현재 seeder가 쓰는 앱 내부 beginner taxonomy가 기존 data 의미와 맞는지 확인한다.
5. [internal/model/question.go:43](../../internal/model/question.go) — JLPT official-style N1 확장용 taxonomy가 빠짐없이 포함되는지 확인한다.
6. [internal/model/question.go:81](../../internal/model/question.go) — `Question.Skill *Skill`가 `db:"item_type"`와 `omitempty` JSON contract를 가지는지 확인한다.

### 3. Persistence Path

1. [internal/repository/question_repo.go:103](../../internal/repository/question_repo.go) — batch insert `columnCount`가 12개로 갱신되어 placeholder와 args 개수가 맞는지 확인한다.
2. [internal/repository/question_repo.go:107](../../internal/repository/question_repo.go) — `INSERT INTO questions` column list에 `item_type`이 `type` 바로 뒤에 포함되는지 확인한다.
3. [internal/repository/question_repo.go:125](../../internal/repository/question_repo.go) — args append 순서가 SQL column 순서와 일치하고 `q.Skill`이 누락되지 않는지 확인한다.

### 4. Seeder Generation

1. [cmd/ja/kana_seeder/main.go:193](../../cmd/ja/kana_seeder/main.go) — kana question 생성 시 `isToRomaji`에 따라 `kana_reading`과 `kana_recall`이 분리되는지 확인한다.
2. [cmd/ja/kana_seeder/main.go:254](../../cmd/ja/kana_seeder/main.go) — kana recall/reading question struct에 `Skill`이 세팅되는지 확인한다.
3. [cmd/ja/kana_seeder/main.go:268](../../cmd/ja/kana_seeder/main.go) — kana handwriting question이 `kana_handwriting` skill을 세팅하는지 확인한다.
4. [cmd/ja/vocab_seeder/main.go:181](../../cmd/ja/vocab_seeder/main.go) — kana-to-meaning vocabulary 객관식이 `vocab_meaning` skill을 세팅하는지 확인한다.
5. [cmd/ja/vocab_seeder/main.go:198](../../cmd/ja/vocab_seeder/main.go) — meaning-to-kana text recall이 `vocab_recall` skill을 세팅하는지 확인한다.
6. [cmd/ja/vocab_seeder/main.go:214](../../cmd/ja/vocab_seeder/main.go) — vocabulary handwriting question이 `vocab_handwriting` skill을 세팅하는지 확인한다.

### 5. Tests

1. [internal/model/question_test.go:5](../../internal/model/question_test.go) — pointer helper가 nullable `Skill` 세팅에 사용할 수 있는 값을 반환하는지 확인한다.
2. [internal/model/question_test.go:12](../../internal/model/question_test.go) — N1 확장용 taxonomy constants가 비어 있지 않은지 최소 회귀 테스트가 있는지 확인한다.
3. [internal/repository/question_repo_test.go:10](../../internal/repository/question_repo_test.go) — batch insert query 테스트가 `item_type` column, placeholder 수, args 수를 검증하는지 확인한다.
4. [internal/repository/question_repo_test.go:57](../../internal/repository/question_repo_test.go) — insert args에서 `item_type` 위치가 SQL column 순서와 맞는지 확인한다.
5. [cmd/ja/kana_seeder/main_test.go:61](../../cmd/ja/kana_seeder/main_test.go) — kana seeder가 reading/recall/handwriting skill을 각각 검증하는지 확인한다.
6. [cmd/ja/vocab_seeder/main_test.go:11](../../cmd/ja/vocab_seeder/main_test.go) — vocabulary meaning question의 skill 검증이 기존 type/category 검증과 함께 있는지 확인한다.
7. [cmd/ja/vocab_seeder/main_test.go:58](../../cmd/ja/vocab_seeder/main_test.go) — vocabulary recall question의 skill 검증이 있는지 확인한다.
8. [cmd/ja/vocab_seeder/main_test.go:82](../../cmd/ja/vocab_seeder/main_test.go) — vocabulary handwriting question의 skill 검증이 있는지 확인한다.

### 6. Runtime Notes

1. [docs/workthrough/2606/2606131243_question_item_type_taxonomy.md:76](../../docs/workthrough/2606/2606131243_question_item_type_taxonomy.md) — targeted tests와 `make test` 검증 기록을 확인한다.
2. [STATUS.md:43](../../STATUS.md) — 최근 완료 항목에 taxonomy/backfill 작업이 기록되어 있는지 확인한다.

## Notes

- `item_type`은 의도적으로 nullable이다. 기존 또는 외부 생성 row의 의미를 모르면 추측 backfill하지 않는 정책이다.
- `questions.type`은 아직 `SessionFlow`, `GraderService`, `HandwritingService`의 hot path에서 rendering/grading mode로 쓰인다. 이번 변경은 해당 hot path를 건드리지 않는 하위호환 변경이다.
- `item_type` 기반 analytics나 N1 generator template selection은 아직 구현되지 않았다.
- 로컬 DB에는 직접 `ALTER TABLE questions ADD COLUMN IF NOT EXISTS item_type VARCHAR(64)`와 backfill이 적용되었다.
