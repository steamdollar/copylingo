# 작업 기록 (2606131243_question_item_type_taxonomy)

## 배경

N1까지 확장하려면 JLPT 공식 문항 taxonomy가 필요하지만, 기존 `questions.type`은 Telegram rendering 및 grading mode로 사용되고 있었다.

예:

- `multiple_choice`: Inline Keyboard 선택지
- `fill_blank`: 텍스트 exact match
- `subjective`: LLM semantic grading
- `kana_handwriting`: Mini App handwriting grading

따라서 `questions.type`을 직접 JLPT taxonomy로 바꾸지 않고, 새 축 `questions.item_type`을 추가했다.

## 결정

- `questions.type`은 기존 풀이/채점 방식으로 유지한다.
- `questions.item_type VARCHAR(64)`를 nullable로 추가한다.
- Go model에는 `model.Skill` enum을 추가한다.
- 기존 로컬 DB row는 현재 의미가 명확한 `type + category` 조합만 backfill했다.

## 변경 파일

- `internal/model/question.go`
  - `Skill` enum 추가
  - `Question.Skill *Skill` 필드 추가
  - `SkillPtr` helper 추가
- `migrations/001_init.sql`
  - `questions.item_type` 컬럼 추가
- `internal/repository/question_repo.go`
  - batch insert 대상 컬럼에 `item_type` 추가
- `cmd/ja/kana_seeder/main.go`
  - kana reading/recall/handwriting skill 세팅
- `cmd/ja/vocab_seeder/main.go`
  - vocabulary meaning/recall/handwriting skill 세팅
- 테스트
  - `internal/model/question_test.go`
  - `internal/repository/question_repo_test.go`
  - `cmd/ja/kana_seeder/main_test.go`
  - `cmd/ja/vocab_seeder/main_test.go`
- `docs/adr/ADR_from_21_to_40.md`
  - ADR-026 추가

## DB 반영

로컬 DB에 직접 반영했다.

```sql
ALTER TABLE questions ADD COLUMN IF NOT EXISTS item_type VARCHAR(64);

UPDATE questions
SET item_type = CASE
    WHEN category = 'kana' AND type = 'multiple_choice' THEN 'kana_reading'
    WHEN category = 'kana' AND type = 'fill_blank' THEN 'kana_recall'
    WHEN category = 'handwriting' AND type = 'kana_handwriting' THEN 'kana_handwriting'
    WHEN category = 'vocabulary' AND type = 'multiple_choice' THEN 'vocab_meaning'
    WHEN category = 'vocabulary' AND type = 'fill_blank' THEN 'vocab_recall'
    WHEN category = 'vocabulary' AND type = 'kana_handwriting' THEN 'vocab_handwriting'
    ELSE item_type
END
WHERE item_type IS NULL;
```

Backfill 결과:

| type | category | item_type | count |
|---|---|---|---:|
| `kana_handwriting` | `handwriting` | `kana_handwriting` | 206 |
| `fill_blank` | `kana` | `kana_recall` | 290 |
| `multiple_choice` | `kana` | `kana_reading` | 126 |
| `fill_blank` | `vocabulary` | `vocab_recall` | 100 |
| `kana_handwriting` | `vocabulary` | `vocab_handwriting` | 100 |
| `multiple_choice` | `vocabulary` | `vocab_meaning` | 100 |

## 검증

```bash
go test ./internal/model ./internal/repository ./cmd/ja/kana_seeder ./cmd/ja/vocab_seeder -v
make test
```

통과.
