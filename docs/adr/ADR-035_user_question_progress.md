# ADR-035: Quiz Question은 공유 Catalog로 유지하고 학습 상태는 사용자별 Progress로 분리

- **날짜**: 2026-07-20
- **상태**: 채택됨

## 맥락

- `questions`는 `language`, `proficiency_level`에 맞는 사용자들이 같은 문항을 여러 Session에서 재사용하는 공유 Catalog다.
- 현재는 공유 콘텐츠와 함께 `ease_factor`, `interval_days`, `repetitions`, `next_review_at`,
  `last_reviewed_at`, `times_served`, `times_correct`가 같은 row에 저장된다.
- 따라서 한 사용자의 답변과 SRS 갱신이 다른 사용자의 신규/복습 문항 판정과 누적 통계에 영향을 준다.
- Material 학습 상태는 이미 `user_material_progress(user_id, material_id)`로 사용자별 분리되어 있으며,
  ADR-024/026/027도 Quiz SRS의 `user_question_progress` 분리를 후속 설계로 명시했다.
- 기존 전역 학습 상태는 현재 실제 owner의 학습 이력이므로, 새 구조로 전환할 때 보존해야 한다.

## 결정

### 데이터 소유권

- `questions`는 공유 Question Catalog로 유지한다. 문항 내용, taxonomy, language/level, Material 연결,
  audio metadata처럼 사용자와 무관한 속성만 소유한다.
- 새 `user_question_progress`가 `(user_id, question_id)`를 Primary Key로 갖고 다음 사용자별 상태를 모두 소유한다.
  - `ease_factor`
  - `interval_days`
  - `repetitions`
  - `next_review_at`
  - `last_reviewed_at`
  - `times_served`
  - `times_correct`
  - `created_at`, `updated_at`
- `session_questions`는 기존처럼 Session별 출제 순서와 실제 답안/정오답 이력을 소유한다.
- `questions.times_served`와 `questions.times_correct`를 전역 aggregate로 남기지 않는다. 콘텐츠 품질용 전역 통계가
  필요해지면 사용자 progress 또는 `session_questions`에서 별도로 집계하며, 공유 Catalog에 mutable 학습 상태를 다시 넣지 않는다.

### 조회와 갱신

- 사용자에게 "신규 문항"은 해당 `(user_id, question_id)` progress row가 없는 문항이다.
- "복습 문항"은 해당 사용자의 `next_review_at <= NOW()`인 progress row를 기준으로 선택한다.
- Session 생성 시 user scope와 함께 Question의 `language`, `proficiency_level` scope를 모두 적용한다.
- 답변 중에는 Redis Working Set의 사용자별 progress copy를 갱신하고, Session 완료 transaction에서
  `user_question_progress`를 upsert한다.
- 사용자 통계는 반드시 `user_id`로 scope한다. 오늘 통계와 이력은 `sessions`/`session_questions`, 누적 Question 통계는
  `user_question_progress`를 기준으로 계산한다.

### Owner backfill

- 기존 `questions`의 전역 학습 상태는 **현재 owner 한 명에게만** backfill한다. 다른 사용자에게 같은 상태를 복제하지 않는다.
- backfill 대상 owner Telegram user ID는 migration 코드에 hardcode하지 않고 실행 시 명시적으로 지정하며,
  해당 `users.id`가 존재하는지 검증한 뒤 실행한다.
- 다음 중 하나 이상의 실제 학습 흔적이 있는 Question만 backfill한다.
  - `times_served > 0`
  - `times_correct > 0`
  - `repetitions > 0`
  - `next_review_at IS NOT NULL`
  - `last_reviewed_at IS NOT NULL`
- 기본값만 가진 미풀이 Question은 progress row를 만들지 않는다. 그래야 owner에게도 계속 신규 문항으로 선택된다.
- backfill은 재실행 가능해야 하며 `(user_id, question_id)` conflict를 안전하게 처리한다.

### 전환과 rollback

- 전환은 additive migration으로 시작한다: table/index 추가 → owner backfill → 새 경로 read/write 전환 → 검증 순서다.
- 안정화 기간에는 legacy `questions` SRS/stat column을 바로 제거하지 않되, cutover 이후에는 read/write하지 않는
  backfill 시점 snapshot으로만 보존한다. 전역 column dual-write는 여러 사용자의 상태를 정확히 표현할 수 없어 금지한다.
- 따라서 cutover 이후 old binary로 복귀하면 새 학습 상태를 잃을 수 있으므로 application rollback은 지원하지 않는다.
  문제 발생 시 새 schema를 유지한 채 forward-fix하며, destructive down migration에 의존하지 않는다.
- cutover 검증에는 owner의 progress row 수, due Question 집합, 신규 Question 집합, Session 완료 후 SRS/stat delta를 포함한다.

## 결과

### 장점

- Question 콘텐츠는 language/level별로 계속 공유하면서 사용자별 SRS와 통계가 서로 오염되지 않는다.
- 사용자 수 증가와 관계없이 학습 상태 ownership이 명확하고, Material/Question progress 모델이 일관된다.
- `session_questions`의 attempt history와 `user_question_progress`의 현재 상태가 분리되어 analytics와 복구 기준이 명확해진다.

### 단점 / 트레이드오프

- due/new Question query에 사용자별 join과 index 설계가 필요하고, progress row 수는 사용자가 실제로 푼 문항 수에 비례해 증가한다.
- `SRSService`, `SessionBuilderService`, Redis Active Session flush, repository, analytics와 테스트가 함께 변경되는 중대형 migration이다.
- 기존 전역 row에는 사용자 provenance가 없으므로 owner 외 사용자에게 과거 학습 상태를 정확히 복원할 수 없다.
- legacy column 제거 전까지 두 schema가 공존하므로 전환 기간의 read/write SSOT를 명확히 지켜야 한다.

## 대안

- **Question row를 사용자마다 복제**: 콘텐츠 중복, seed/update drift, audio 중복이 발생해 공유 Catalog 목적을 깨므로 기각.
- **SRS만 사용자별로 옮기고 served/correct는 전역 유지**: 콘텐츠 통계와 사용자 학습 통계의 의미가 다시 섞이므로 기각.
- **`session_questions` 이력만으로 현재 SRS를 매번 재구성**: append-only 이력은 보존되지만 hot-path due query와 Session 생성 비용이 커져 기각.
- **현 전역 SRS 유지**: 사용자 간 학습 상태 오염이 수만 사용자 가정과 충돌하므로 기각.
