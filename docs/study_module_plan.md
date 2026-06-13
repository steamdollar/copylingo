# Study Module 단계별 도입 Plan

> 작성일: 2026-06-02  
> 상태: Task 1~3 및 정오 Scheduler 연동 구현 완료. Task 4는 별도 설계 합의 전까지 보류한다.

## 배경

현재 `questions`는 Quiz 출제 단위이며,  사용자가 먼저 학습하는 흐름이 부재.
Study Session과 Quiz Session은 분리하되, 기존 Quiz 경로를 유지하면서 Study Module을 단계적으로 추가한다.
외부 수집 원문인 `contents`와 실제 학습 단위인 `materials`는 서로 다른 lifecycle을 가지므로 별도 테이블로 관리한다.

## Task 1. `materials` 테이블 디자인 및 추가

- 상태: 구현 완료 (`materials` Schema, Model, Repository, Vocabulary 전용 Material Seeder).
- 목표: 사용자가 학습할 카드 단위의 SSOT를 추가한다.
- `contents`는 NHK 등 외부 수집 원문으로 유지하고, `materials.content_id`는 nullable FK로 둔다.
- 초기 컬럼 후보:
  - `id SERIAL PRIMARY KEY`
  - `material_key VARCHAR(255) NOT NULL UNIQUE`
  - `content_id INT NULL REFERENCES contents(id) ON DELETE SET NULL`
  - `category VARCHAR(30) NOT NULL`, `language VARCHAR(10) NOT NULL`, `proficiency_level VARCHAR(10) NOT NULL`
  - `title TEXT NOT NULL`, `payload JSONB NOT NULL DEFAULT '{}'`, `difficulty INT NOT NULL`, `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- `material_key`는 Seeder 재실행 시 Idempotency 보장에 사용한다.
- Naming Convention은 `{language}:{domain}:{stable_slug}` 형식으로 고정한다.
- 예시: `ja:kana:u3042`, `ja:vocab:word_024`, `ja:grammar:particle_wo`.
- Level은 변경 가능한 Metadata이므로 Key에 넣지 않고 `proficiency_level` 컬럼으로만 관리한다.
- `payload`는 가나, 단어, 문법, 문장처럼 형태가 다른 카드 데이터를 수용한다.
- Migration은 기존 SQL 파일에 `IF NOT EXISTS`를 포함해 추가한다.
- `cmd/ja/material_seeder`는 기존 Question Seeder와 독립적으로 Vocabulary Material만 Upsert한다.
- 초기 repo-owned curated N5 Vocabulary Catalog는 500개다.
- Kana Material, Grammar Material, Sentence Material은 Study UX 우선순위에 맞춰 후속 범위로 보류한다.

## Task 2. 공용 `sessions` 확장

- 상태: 구현 완료 (`sessions.mode`, `session_materials`, `user_material_progress` Schema/Model/Repository).
- 목표: 별도 `study_sessions` 없이 기존 `sessions`를 Study와 Quiz의 공통 상위 테이블로 사용한다.
- 권장안: `sessions.mode VARCHAR(20) NOT NULL`를 추가하고 application layer의 `SessionMode` enum으로 명시적으로 세팅한다.
- 기존 `sessions.type`은 `morning`, `evening`, `review`, `article` 같은 목적을 유지한다.
- `mode`는 `study`, `quiz`처럼 상호작용 방식을 구분한다.
- Quiz는 기존 `session_questions`를 그대로 사용한다.
- Study는 신규 `session_materials` Join Table을 사용한다.
- `session_materials` 초기 컬럼: `id`, `session_id`, `material_id`, `material_order`, `studied_at`.
- Material 반복 학습 상태는 `user_material_progress`에 user별 SRS state로 저장한다.
- `user_material_progress` 초기 컬럼: `user_id`, `material_id`, `ease_factor`, `interval_days`, `repetitions`, `next_review_at`, `last_studied_at`, `times_studied`.
- `UNIQUE (session_id, material_id)`와 `UNIQUE (session_id, material_order)`를 둔다.
- Migration은 Task 1과 동일.
- 기존 Quiz Session은 backfill된 `mode='quiz'` 값으로 하위 호환성을 유지한다.

## Task 3. Study Session 최소 동작 확인

- 상태: 구현 완료 (`StudySessionService`, `StudyActiveSessionService`, Telegram `StudyFlow`, 정오 Scheduler Push).
- 목표: Material이 실제 Study Session에서 순서대로 노출되고 완료 기록이 남는 Vertical Slice를 만든다.
- `StudySessionService`가 Study Session과 ordered `session_materials`를 생성한다.
- 정오 Study Session은 Vocabulary Material 8개를 노출한다. Kana 단일 문자 Material은 초기 학습 효용이 낮아 후보에서 제외한다.
- 별도 `StudyFlow`가 첫 Material 표시, 다음 카드 이동, Session 완료를 담당한다.
- 초기 카드는 `title`과 `payload`를 단순 렌더링하며 고급 UX는 보류한다.
- Start 시 DB의 `sessions`와 ordered `session_materials`에서 Study Redis Working Set을 생성한다.
- `다음` 동작 시 DB write 없이 Redis의 `StudyActiveSessionState`에서 해당 Material만 studied 처리한다.
- 마지막 카드 처리 시 `sessions.status='completed'`, `session_materials.studied_at`, `user_material_progress`를 transaction으로 flush한다.
- Flush 대상 progress는 Working Set 로드 시 이미 studied였던 Material을 제외한 신규 학습 Material로 제한한다.
- Redis get/save/delete 공통 로직은 Quiz와 Study가 generic `workingSetStore[T]`로 공유하되, domain service는 `ActiveSessionService`와 `StudyActiveSessionService`로 분리한다.
- Scheduler 자동 생성과 Push는 정오 Study Session 요구사항에 맞춰 함께 구현했다.
- “오늘 학습량” 계산은 아직 구현하지 않는다.
- 우선 수동 Trigger 또는 Test Helper로 Study Session 생성 경로를 검증한다.
- Service, Repository, Bot Flow Test를 추가하고 `make test`를 실행한다.

## Task 4. `questions`와 `materials` 연결 검토

- 상태: 기록만 남기며 구현하지 않는다.
- 후보 변경: `questions.material_id INT NULL REFERENCES materials(id) ON DELETE SET NULL`.
- 목적: 하나의 Material에 객관식, 주관식, 손글씨 등 여러 Quiz 표현을 연결한다.
- 향후 검토: Study한 Material 중 일부를 당일 Quiz Session에 포함할지 결정한다.
- 향후 검토: 새 문제와 복습 문제의 비율, 출제 시점, Scheduler 연결 방식을 결정한다.
- 향후 검토: SRS SSOT를 `(user_id, question_id)` 또는 `(user_id, material_id)` 중 어디에 둘지 결정한다.
- 향후 검토: 기존 Question 데이터 Backfill과 Seeder 구조 변경 범위를 정한다.
- 위 결정은 별도 ADR 합의 후 구현한다.

## 권장 구현 순서

1. Task 1: Material SSOT 추가
2. Task 2: 공용 Session 확장
3. Task 3: Study Session Vertical Slice 검증
4. 실제 사용 흐름 확인 후 Task 4 ADR 논의
