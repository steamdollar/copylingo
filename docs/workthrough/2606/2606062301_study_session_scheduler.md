# 작업 기록 (2606062301_study_session_scheduler)

## 배경

정오쯤 사용자 레벨에 맞는 Study Material Session을 Telegram으로 Push하는 흐름을 구현했다.
기존 `materials`는 SSOT와 seeder만 있었고, 사용자 session lifecycle과 연결되지 않은 상태였다.

## 결정 사항

- 기존 `sessions`를 Quiz/Study 공통 parent로 유지한다.
- `sessions.mode`로 `quiz`/`study` interaction model을 구분한다.
- Study child table은 `session_materials`로 추가한다.
- `session_materials.session_id`는 owned child라 `ON DELETE CASCADE`를 사용한다.
- `session_materials.material_id`는 catalog 참조라 CASCADE 없이 FK만 둔다.
- Study 진행 상태는 `session_materials.studied_at`으로 기록한다.
- Material 반복 학습 SRS state는 `user_material_progress`가 user별로 소유한다.
- Study Session 생성은 due Material을 우선하고, 부족한 슬롯은 신규 Material로 채운다.
- Quiz Redis Active Session Working Set은 Study flow에서 재사용하지 않는다.

## 변경 파일

- `migrations/001_init.sql`
  - `sessions.mode VARCHAR(20) NOT NULL` 추가
  - `session_materials` CREATE 구문 추가
  - `user_material_progress` CREATE 구문 추가
- `internal/model/session.go`
  - `SessionMode`, `SessionStudy`, `SessionMaterial`, `StudySessionMaterial` 추가
- `internal/model/material.go`
  - `UserMaterialProgress` 추가
- `internal/repository/material_repo.go`
  - 사용자별 due/new Material 조회 추가
- `internal/repository/material_progress_repo.go`
  - user별 Material SRS upsert 추가
- `internal/repository/session_material_repo.go`
  - Study session material 생성, 조회, idempotent studied mark 추가
- `internal/repository/session_repo.go`
  - `CreateSession`에 mode validation 및 저장
  - repository error wrapping 정리
  - restart recovery용 `ListInProgress`는 Quiz만 조회하도록 제한
- `internal/service/study_session.go`
  - `BuildStudySession`, `StartSession`, `MarkStudied`, `CompleteSession` 추가
  - `MarkStudied`에서 새로 완료된 Material만 `user_material_progress` 갱신
- `internal/bot/study_flow.go`
  - Study start/next/finish callback 처리
  - Vocabulary Material Telegram rendering 추가
- `internal/scheduler/scheduler.go`
  - `study_push` job 추가
- `internal/config/config.go`, `config.yaml`
  - `study_push_cron` 추가, 기본값 `0 12 * * *`
- `docs/adr/ADR_from_21_to_40.md`
  - ADR-024 추가
- `docs/study_module_plan.md`
  - Task 2/3 완료 상태 반영

## 로컬 DB 직접 변경

사용자 요청에 따라 별도 migration 파일을 추가하지 않고, 로컬 DB에는 직접 `psql`로 적용했다.

```bash
PGPASSWORD=copylingo psql -h localhost -U copylingo -d copylingo -v ON_ERROR_STOP=1 -c "ALTER TABLE sessions ADD COLUMN IF NOT EXISTS mode VARCHAR(20) NOT NULL DEFAULT 'quiz'; CREATE TABLE IF NOT EXISTS session_materials (id SERIAL PRIMARY KEY, session_id INT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE, material_id INT NOT NULL REFERENCES materials(id), material_order INT NOT NULL, studied_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), UNIQUE (session_id, material_id), UNIQUE (session_id, material_order)); CREATE INDEX IF NOT EXISTS idx_session_materials_session_id ON session_materials(session_id);"
```

결과:

```text
ALTER TABLE
CREATE TABLE
CREATE INDEX
```

이후 `mode`를 DB default가 아니라 application layer enum으로 소유하도록 조정했다. 기존 row는 이미 `quiz` 값으로 채워져 있으므로 default만 제거했다.

```bash
PGPASSWORD=copylingo psql -h localhost -U copylingo -d copylingo -v ON_ERROR_STOP=1 -c "ALTER TABLE sessions ALTER COLUMN mode DROP DEFAULT;"
```

추가 확인:

```text
sessions.mode = character varying DEFAULT <NULL>
existing sessions.mode = quiz 164 rows
sessions.mode IS NULL = 0 rows
session_materials columns = id, session_id, material_id, material_order, studied_at, created_at
session_materials_session_id_fkey delete action = CASCADE
session_materials_material_id_fkey delete action = NO ACTION
```

Material 반복 학습을 위해 user별 SRS table을 추가로 적용했다.

```bash
PGPASSWORD=copylingo psql -h localhost -U copylingo -d copylingo -v ON_ERROR_STOP=1 -c "CREATE TABLE IF NOT EXISTS user_material_progress (user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE, material_id INT NOT NULL REFERENCES materials(id), ease_factor DOUBLE PRECISION NOT NULL DEFAULT 2.5, interval_days INT NOT NULL DEFAULT 0, repetitions INT NOT NULL DEFAULT 0, next_review_at TIMESTAMPTZ, last_studied_at TIMESTAMPTZ, times_studied INT NOT NULL DEFAULT 0, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), PRIMARY KEY (user_id, material_id)); CREATE INDEX IF NOT EXISTS idx_user_material_progress_due ON user_material_progress(user_id, next_review_at) WHERE next_review_at IS NOT NULL;"
```

결과:

```text
CREATE TABLE
CREATE INDEX
completed session_materials rows = 0
user_material_progress rows = 0
```

## 검증

```bash
go test ./internal/service ./internal/bot
go test ./...
```

결과: PASS.

## 후속 검토

- `sessions.total_questions`, `correct_count`는 Study에는 명칭이 어색하다. Breaking Change 없이 우선 유지했고, analytics 확장 시 `total_items` 또는 mode별 summary 구조를 검토한다.
- 기존 Question SRS는 `questions` row에 state가 있어 user별 SRS가 아니다. 향후 `user_question_progress` 분리를 검토한다.
- 오늘 Study 완료량 통계는 아직 구현하지 않았다.
- `questions.material_id` 연결과 Study Material 기반 Quiz 생성은 Task 4 범위로 보류한다.
