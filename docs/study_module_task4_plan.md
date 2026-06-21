# Study Module Task 4 Seed Rebuild Plan

> 작성일: 2026-06-15  
> 상태: 구현 및 로컬 DB 재생성 완료  
> 범위: `questions.material_id` 연결, JA seed catalog 공용화, user 보존 DB reset, Material/Question/Study Session 재생성, Question seed 멱등성.

## 1. 결론

Task 4는 Backfill 방식이 아니라 **seed source를 정리한 뒤 DB를 재생성**하는 방식으로 처리한다.

`users`는 유지하고, 학습 데이터는 초기화한다.
이후 공용 JA seed catalog에서 Kana/Vocabulary Material을 만들고, Question Seeder가 해당 Material ID를 들고 Question을 생성한다.

## 2. 목표

- `questions.material_id` nullable FK를 유지한다.
- `questions.question_key` stable key로 seed Question upsert를 보장한다.
- Kana/Vocabulary seed catalog를 `cmd/ja`로 통합한다.
- Material Seeder가 Kana Material + Vocabulary Material을 모두 upsert한다.
- Kana/Vocabulary Question Seeder가 Material lookup 후 `material_id`를 세팅한다.
- Kana/Vocabulary Question Seeder는 deterministic generation을 사용한다.
- Backfill command는 제거한다.
- DB reset command는 `users`를 보존한다.
- reset 후 Material, Question, pending Study Session row를 재생성한다.

## 3. 실행 순서

```bash
go run ./cmd/admin/reset_learning_data -yes
docker compose exec -T redis redis-cli -n 0 FLUSHDB
go run ./cmd/ja/seeder
go run ./cmd/admin/build_study_sessions
make restart-app
```

## 4. 완료 기준

- `users` row가 유지된다.
- `materials`에는 `kana=208`, `vocabulary=500`이 존재한다.
- `questions`에는 `kana=416`, `handwriting=206`, `vocabulary=1500`이 존재한다.
- 모든 seed Question은 `material_id IS NOT NULL`이다.
- 모든 seed Question은 `question_key IS NOT NULL`이고 중복이 없다.
- pending Study Session이 user별로 생성되고 `session_materials` 8개가 연결된다.
- `make test` 통과.
- `http://localhost:8080/health` ready.

## 5. 후속 후보

- Study 완료 Material을 Evening Quiz 편성에 반영.
- Quiz SRS를 user별 `user_question_progress`로 분리.
