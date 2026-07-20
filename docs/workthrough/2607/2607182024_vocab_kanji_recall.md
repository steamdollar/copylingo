# Vocabulary 한자 recall 문항 + 세션당 3개 cap

## 배경

- 기존 N5 Vocabulary Material의 `kana`, `kanji`, `meaning_ko`를 재사용해 한자 표기를 직접 입력하는 Quiz를 추가했다.
- 한 세션에 한자 직접 입력이 너무 많아지지 않도록 morning/evening/review 전체의 최대치를 3개로 제한했다.

## 결정

- ADR-026의 축 분리를 따라 rendering/grading mode는 기존 `fill_blank`를 재사용하고, 새 skill `vocab_kanji_recall`만 추가했다.
- 문항 category는 개별 한자 읽기가 아닌 vocabulary orthography recall이므로 `vocabulary`로 유지했다.
- 생성 자격은 `Kanji != Kana && unicode.Han 1개 이상`으로 고정했다. 현재 540개 중 431개다.
- Stable key는 `ja:vocab:<dataset_id>:kanji_recall`, 정답은 catalog `Kanji`, `material_id`는 기존 vocabulary material을 재사용했다.
- Repository가 `LIMIT` 전에 남은 kanji budget을 적용해 non-kanji로 backfill하고, Session Builder admission gate가 세션 최종 cap을 이중 검증한다.
- 상세 결정은 `docs/adr/ADR_from_21_to_40.md` ADR-033에 기록했다.

## 변경 파일

- `cmd/ja/seeder/main.go`, `main_test.go`: 한자 recall builder·eligibility·stable key·431개 catalog 무결성 테스트.
- `internal/model/question.go`, `question_test.go`: `SkillVocabKanjiRecall` taxonomy.
- `internal/repository/question_repo.go`, `question_repo_test.go`: new/due query의 budget-aware window rank와 non-kanji backfill.
- `internal/service/srs.go`, `srs_test.go`: due review kanji budget 전달.
- `internal/service/session_builder.go`, `session_builder_test.go`: 남은 budget 전달과 세션당 admission cap 3.
- `internal/service/grader_test.go`, `internal/bot/*_test.go`: 변경된 internal contract에 맞춘 mock.
- `docs/adr/ADR_from_21_to_40.md`: ADR-033.

## 위임·리뷰

- Reader `/root/kanji_question_inventory` (`gpt-5.6-luna`, low, read-only): catalog/question/session 경로를 일괄 분류. 대량 탐색을 짧은 evidence digest로 줄여 profitable, context hygiene cleaner.
- Advisor `/root/kanji_question_design_review` (`gpt-5.6-sol`, high, read-only): type/skill/category와 query-side cap tradeoff 검토.
- Primary가 seed builder, repository SQL, session admission path, 테스트를 직접 재검증했다. 위임 deviation는 없다.

## 검증·반영

- Targeted: `go test ./cmd/ja/seeder ./internal/model ./internal/repository ./internal/service ./internal/bot` → PASS.
- PostgreSQL new/due CTE·window query `EXPLAIN` → 두 query 모두 문법/계획 생성 성공.
- `git diff --check` → PASS.
- `make test` → PASS.
- `make restart-app` → `http://localhost:8080/health` ready.
- DB target: local Docker `copylingo-db`, database `copylingo`.
- 사전 backup: `/tmp/copylingo-kanji-recall.OlGa7k/copylingo-before-kanji-recall.dump` (`pg_restore -l` 검증, 201080 bytes).
- Seeder 1차: Material 828개, Question 2,878개 upsert. `vocab_kanji_recall=431`, distinct key 431, invalid row 0.
- Seeder 2차: Question 2,878개·`vocab_kanji_recall=431` 유지, duplicate question key 0 → 멱등성 확인.
- 실제 new vocabulary candidate query(`limit=100`, kanji budget 3): total 100, `vocab_kanji_recall=3`.

## Runtime/회복

- Schema 변경은 없다. Seed upsert는 `times_served`, `times_correct`, SRS 필드를 갱신하지 않아 기존 runtime state를 보존한다.
- 즉시 rollback이 필요하면 dependent session이 생성되기 전 `item_type='vocab_kanji_recall'` + stable-key suffix로 431개만 제거하거나 사전 dump를 복원한다. 실제 rollback은 수행하지 않았다.
