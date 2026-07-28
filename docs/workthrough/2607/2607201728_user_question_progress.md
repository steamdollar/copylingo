# Quiz 사용자별 Question Progress 분리

## 목적

공유 `questions` row가 전역 SRS와 served/correct 통계를 함께 소유해 사용자 간 학습 상태가 섞이던 구조를
ADR-035에 따라 `(user_id, question_id)` 기반 `user_question_progress`로 전환했다.

## 결정

- `questions`는 language/level별 공유 Question Catalog만 표현한다.
- SRS와 `times_served`/`times_correct`의 read/write SSOT는 `user_question_progress`다.
- 기존 global column은 backfill snapshot으로만 보존하며 dual-write하지 않는다.
- due/new 조회와 통계는 모두 user scope를 적용하고, due에는 현재 language/level도 적용한다.
- Redis Active Session version을 2로 올리고 Question과 Progress copy를 분리했다.

## 구현

- Schema/운영: `migrations/001_init.sql`, `scripts/backfill_user_question_progress.sql`, `Makefile`,
  `cmd/admin/reset_learning_data/main.go`
- Model: `internal/model/question.go`, `question_progress.go`, `active_session.go`
- Repository: `question_repo.go`, `active_session_repo.go`, `session_question_repo.go`
- Service/Bot: SRS scheduling, Session Builder, Active Session flush, analytics, review/menu user scope
- 문서: `docs/ARCHITECTURE.md`, ADR-035 및 ADR range pointer
- 관련 model/repository/service/bot test와 mock 계약을 함께 갱신했다.

## Local DB cutover

- 대상 owner: Telegram user `2006481393`
- 적용 순서: App 중지 → additive schema 적용 → owner backfill → App restart
- legacy 학습 흔적 429건을 backfill했고 새 row와 legacy snapshot의 7개 progress field mismatch는 0건이다.
- owner의 현재 JA/N5 due는 151건, progress row가 없는 new Question은 2,499건이다.
- backfill 재실행 결과 `INSERT 0 0`, 최종 row 429건으로 멱등성을 확인했다.
- 첫 `make migrate`는 password 미전달로 인증 전에 실패해 DB 변경이 없었다.
- 첫 backfill은 PostgreSQL temp→persistent FK 제한으로 transaction rollback됐고, owner 존재 검증을
  psql `EXISTS` gate로 수정한 뒤 성공했다.

## 검증

- `make test`: PASS
- `git diff --check`: PASS
- `http://localhost:8080/health`: healthy
- restart 후 PostgreSQL/Redis/Telegram 연결과 Scheduler 등록 정상, runtime error 없음
- 진행 중 Quiz Session 0건을 확인한 뒤 cutover했다.

## Rollback

Schema와 legacy snapshot은 제거하지 않았다. 새 progress를 global column에 dual-write하지 않으므로 old binary
rollback은 지원하지 않으며, 장애 시 새 schema를 유지한 채 forward-fix한다.
