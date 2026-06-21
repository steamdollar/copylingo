# Study Module Task 4: Question-Material Seed 재생성

## 배경

Task 1~3으로 Study Session은 동작하지만, Quiz Question이 어떤 Study Material에서 파생됐는지 추적할 수 없었다.
초기에는 기존 row를 backfill하는 방식으로 연결했지만, 더 적절한 방향은 seed source 자체를 정리하는 것이다.

기존 문제:

- Vocabulary Material Seeder는 N5 단어 500개를 갖고 있었다.
- Vocabulary Question Seeder는 N5 단어 100개를 별도로 갖고 있었다.
- Kana Material은 아직 생성되지 않았다.
- Backfill command는 과거 row 보정에는 유용하지만, Seeder drift를 막지 못한다.

## 결정

- `questions.material_id` nullable FK를 유지한다.
- JA seed catalog를 `cmd/ja`로 통합한다.
- `cmd/ja`가 Kana map, N5 Vocabulary 500개, Material builder, stable key helper를 소유한다.
- JA Seeder는 Kana Material + Vocabulary Material을 모두 upsert한다.
- JA Seeder는 Material을 먼저 조회하고 기존 Kana/Vocabulary Question 유형을 생성한 뒤 `material_id`를 세팅한다.
- Backfill command는 제거한다.
- 로컬 DB는 `users`만 보존하고 학습 데이터는 reset 후 재생성한다.
- reset 후 현재 user에 pending Study Session row도 재생성한다.

## 변경 파일

- `cmd/ja/`
  - Kana map, Vocabulary catalog 500개, Material builder 추가.
- `cmd/ja/seeder/main.go`
  - Material 생성 후 Kana/Vocabulary Question 생성을 한 흐름으로 통합.
  - Kana/Vocabulary Material lookup 후 `material_id` 세팅.
- `cmd/admin/reset_learning_data`
  - `users`를 제외한 학습 데이터 table reset command 추가.
- `cmd/admin/build_study_sessions`
  - 기존 `StudySessionService`로 user별 pending Study Session 생성 command 추가.
- `cmd/ja/backfill_question_materials`
  - 제거.
- `docs/adr/ADR_from_21_to_40.md`
  - ADR-027 갱신.
- `docs/study_module_plan.md`, `docs/study_module_task4_plan.md`
  - reset/seed 재생성 방식으로 갱신.

## 실행 결과

테스트:

```bash
make test
```

통과.

DB reset 전 user count:

```text
users_before_reset = 1
```

실행:

```bash
go run ./cmd/admin/reset_learning_data -yes
docker compose exec -T redis redis-cli -n 0 FLUSHDB
go run ./cmd/ja/seeder
go run ./cmd/admin/build_study_sessions
make restart-app
```

결과:

```text
Reset learning data tables while preserving users
Successfully upserted 708 Japanese materials.
Successfully inserted 622 Kana questions.
Successfully inserted 1500 vocabulary questions.
Created 1 study sessions for 1 users.
app is ready: http://localhost:8080/health
```

DB 확인:

```text
users_after_reset = 1

materials:
- kana = 208
- vocabulary = 500

questions:
- handwriting = 206 / with_material = 206
- kana = 416 / with_material = 416
- vocabulary = 1500 / with_material = 1500

sessions:
- study / study / pending = 1

session_materials = 8
```

## 주의 사항

- `cmd/ja/seeder`의 Question 생성은 아직 batch insert다. reset 없이 재실행하면 중복 Question row가 생긴다.
- 운영 DB에는 이 reset 절차를 그대로 적용하면 안 된다. 현재 절차는 로컬 seed 재생성용이다.
- Study Session은 row만 생성했다. Telegram push는 기존 Scheduler 또는 bot flow가 담당한다.
