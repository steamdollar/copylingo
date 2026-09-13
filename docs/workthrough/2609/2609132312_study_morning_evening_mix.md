# Study 아침·저녁 분량 및 신규·복습 편성

## 요청과 결정

- 아침·저녁 각 30분을 기준으로 실제 Study·Quiz 기록을 분석한 뒤, 아침 20개·저녁 24개 구성을 사용자와 확정했다.
- 아침: 단어 15(신규 8/복습 7), 문법 4(1/3), 독해 1(1/0).
- 저녁: 단어 18(4/14), 문법 4(1/3), 독해 2(0/2).
- 상세 선택·보충 정책: [ADR-047](../../adr/ADR-047_study_morning_evening_mix.md).

## 변경 범위

- `internal/model/study_session.go`, `internal/service/study_session.go` 및 테스트: 명시적 Study plan, 아침·저녁 profile, 수동 limit 배분. `/study` 기본값은 20개이며 1~50개를 허용한다. 50개 요청은 단어 38·문법 10·독해 2개로 배분한다.
- `internal/repository/material_repo.go` 및 테스트: 한 SQL에서 유형별 신규·복습 할당, 현재 레벨 신규 우선, due 보충과 미완료 Material 제외. 격리된 transaction 검증을 위해 `SelectContext`/`ExecContext`의 작은 DB interface를 사용한다.
- `internal/scheduler/scheduler.go`, `session_reminder_test.go`: 두 Study cron에 서로 다른 profile을 전달하고 backlog 규칙을 유지한다.
- `internal/config/config.go`, `config_test.go`, `config.yaml`: 사용자 수정값을 보존하면서 YAML과 기본값의 실제 일정 정합성을 맞췄다. 파일·환경 변수 우선순위도 검증한다.
- `internal/bot/study_flow.go` 및 관련 테스트: 시간대를 정오로 고정한 Study 안내를 일반 Study 안내로 바꾸고 변경된 store 계약을 테스트 stub에 반영했다.
- `STATUS.md`, ADR-047 및 ADR range pointer를 갱신했다.

## 검증

- 현재 PostgreSQL에서 실제 선택 SQL을 `BEGIN READ ONLY`로 실행했다. 세션 생성이나 학습 이력 변경은 하지 않았다.
  - 아침: N4 단어 신규 8 + N5 단어 복습 7, N4 문법 신규 1 + 복습 3, N4 독해 신규 1 = 20개.
  - 저녁: N4 단어 신규 4 + N5 단어 복습 14, N4 문법 신규 1 + 복습 3, N5 독해 복습 2 = 24개.
  - 두 profile은 같은 DB snapshot에서 각각 조회했다. 두 세션을 연달아 실제 생성한 검증은 아니다.
- `COPYLINGO_TEST_DATABASE_URL`을 설정한 `make test`: 최종 전체 통과(테스트가 있는 16개 package).
  - `TestGetForStudySessionPostgres`는 실제 PostgreSQL의 TEMP table과 rollback transaction에서 통과했다. 원래 catalog·진행도·세션 행을 수정하지 않았다.
  - profile별 quota, 신규 현재 레벨 우선, 미완료·미래 복습 제외, 부족분 due 보충, 추가 신규 없이 짧은 세션 반환을 검증했다.
  - 최초 전체 실행에서 `/study 50`의 기대값 산술 오류가 발견돼 기대값을 V38/G10/R2로 수정한 뒤 전체 통과를 확인했다.
  - scheduler profile 테스트는 cron Entry.ID 순서로 실행해 현재 시각에 의존하지 않는다.
- `gofmt -l` 변경 Go 파일: 출력 없음. `git diff --check`: 통과.
- `make restart-app`: 성공. 2026-09-13 23:29 KST에 `/health`의 `healthy` 응답을 확인했다.
- 재시작 로그에서 Study 08:00·16:30, Quiz 12:00·21:00 cron 등록을 확인했다.
- 새 세션의 Telegram 실제 학습 완료는 자동 실행하지 않았다. 새 편성은 이후 생성되는 Study 세션부터 적용된다.

## 남는 범위

- N3 콘텐츠 공급·레벨 진단, Quiz 단어 중복 제한, Study/Quiz SRS 연계는 이번 Study 편성 변경에 포함하지 않는다.
- 카드 완료 간격은 실제 집중 시간과 다르므로 첫 주 사용 후 분량을 재조정한다.
