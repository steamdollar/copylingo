# 레거시 정적 스터디 Cron 등록 차단 및 스케줄러 설정 정리

## 1. 배경 및 문제 상황
- **현상**:
  - 사용자가 오후 학습(`evening_study_time`)을 21:00(9pm)으로 설정했음에도 불구하고, 매일 16:30(4:30pm)에 퀴즈 세션과 함께 스터디 세션이 중복 발송됨.
- **원인 분석**:
  - ADR-048 동적 30분 개인화 푸시 엔진(`DynamicPushCron: "*/30 * * * *"`)이 활성화되어 있어 16:30에 `evening_quiz_time`에 따른 동적 Quiz 푸시가 정상 트리거됨.
  - 그러나 [config.yaml](../../config.yaml)에 과거 정적 스케줄링 설정인 `afternoon_study_push_cron: "30 16 * * *"`이 남아있었고, [internal/scheduler/scheduler.go](../../internal/scheduler/scheduler.go)에서 동적 푸시 활성화 여부와 무관하게 무조건 `cron.AddFunc`에 등록되고 있었음.
  - 이로 인해 16:30에 사용자 개인 설정과 상관없이 `GetAllUsers()`를 대상으로 오후 Study 세션이 강제 일괄 발송되어 **동적 Quiz + 레거시 Study**가 동시에 수신됨.

---

## 2. 해결 내용

### 1) [internal/scheduler/scheduler.go](../../internal/scheduler/scheduler.go)
- `Start()` 메서드 내 레거시 크론(`StudyPushCron`, `AfternoonStudyPushCron`) 등록 블록에 `if s.cfg.Schedule.DynamicPushCron.IsZero()` 가드 추가.
- 동적 개인화 푸시 엔진이 활성화되어 있을 경우, 레거시 정적 크론이 절대 등록되지 않도록 원천 차단.

### 2) [config.yaml](../../config.yaml)
- `schedule` 섹션에서 이미 동적 엔진으로 대체된 레거시 크론 항목(`morning_build_cron`, `morning_push_cron`, `study_push_cron`, `afternoon_study_push_cron`, `evening_build_cron`, `evening_push_cron`) 값을 공백(`""`)으로 초기화하고 주석 추가.

### 3) 단위 테스트 추가 ([internal/scheduler/scheduler_test.go](../../internal/scheduler/scheduler_test.go))
- `TestStart_DynamicPushDisablesLegacyStudyCrons`:
  - `DynamicPushCron`이 활성화되어 있을 때 레거시 study 크론이 스케줄러에 등록되지 않고 1개(`dynamic_user_push`)만 등록됨을 검증.
  - `DynamicPushCron`이 비어있을 때는 레거시 study 크론 2개가 정상 등록됨을 검증.

---

## 3. 검증
- `go test -v ./internal/scheduler/...` 단위 테스트 통과
- `go test ./...` 전체 테스트 통과
