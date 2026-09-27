# 사용자별 단일 cron과 미완료 세션 발송 규칙 정리

2026-09-26 17:55 단일 cron 정리와 18:01 미완료 세션 상한 설정 제거를 통합한 기록이다.

## 단일 cron — ADR-052

- 사용자별 네 슬롯의 기본 시각과 변경 값은 DB에 두고, 서버는 매시 `:00`·`:30`에 `tick`을 실행해 해당 시각의 사용자만 조회한다. 결정은 [ADR-052](../../adr/ADR-052_single_push_cron.md)에 기록했다.
- `internal/scheduler/scheduler.go`: `dynamic_user_push` 하나만 등록하도록 바꿨다. 기존 오전·오후 Study 발송, 동적 푸시 부재 시 fallback, 03:00 콘텐츠 자동 수집 등록을 제거했다. 테스트에서만 쓰이던 옛 전역 세션 발송 메서드도 삭제했다.
- `internal/config/config.go`, `internal/config/load.go`, `config.yaml`: 발송·콘텐츠 cron 설정과 `CronExpr` 타입을 제거했다. 이때 남긴 미완료 세션 상한 설정은 18:01 후속 작업에서 제거했다.
- `internal/scheduler/scheduler_test.go`, `internal/scheduler/dispatcher_test.go`, `internal/config/config_test.go`: 단일 30분 등록을 검증하고, 제거된 전역 발송 경로의 테스트를 정리했다. 옛 경로 전용 `session_reminder_test.go`는 삭제하고 현재 dispatcher 테스트가 쓰는 stub만 옮겼다.
- `docs/ARCHITECTURE.md`: 실제 사용자별 발송 흐름으로 갱신했다. 새 cron을 추가하도록 하던 `docs/todos/decouple_tip_audio_topup_cron.md`와 `STATUS.md` 항목은 ADR-052와 충돌해 제거했다.

## 미완료 세션 상한 — ADR-053

- [ADR-053](../../adr/ADR_from_41_to_60.md#adr-053-미완료-세션-상한을-발송-규칙으로-고정한다)에 따라 사용자별 미완료 세션이 3개 이상이면 기존 세션을 재알림하는 규칙을 유지하고, 서버 시작 시 검증하거나 환경변수로 조정하던 설정을 제거했다. 사용자별 상한 조정은 제공하지 않는다.
- `internal/config/config.go`, `internal/config/load.go`, `config.yaml`: 마지막 `ScheduleConfig` 필드·타입, 기본값, 환경변수 연결, YAML 항목을 제거했다. 범위 검증은 이 작업 시작 전에 이미 제거돼 있었다.
- `internal/scheduler/dispatcher.go`: 기존 기본값 3을 발송 규칙 상수로 남겼다. `internal/scheduler/scheduler.go`, `cmd/server/server.go`에서도 사용하지 않는 설정 의존성과 생성자 인자를 제거했다.
- 설정·dispatcher·scheduler 테스트 입력을 정리하고, ADR-045에 설정 방식 변경을 기록했다.

## 유지한 코드와 동작

- 콘텐츠 수집 파이프라인, `initPipeline`, `Scheduler.collectContent`는 재도입에 대비해 유지했다. 자동 수집은 중단됐고, 이후 [서버 시작 경계 정리](./2609261832_server_startup_service_boundary.md)에서 미사용 파이프라인의 초기화 호출도 중단했다.
- Tip·Audio 보충은 현재대로 사용자별 `tick`에서 발송 대상이 있을 때 실행한다. 분리 방식은 새 cron을 추가하지 않는 방향으로 향후 결정한다.
- 사용자별 시각, 시간대, 슬롯 활성화 여부, Redis 중복 발송 방지는 변경하지 않았다.

## 검증

- 17:55 단일 cron 변경 후 `go test ./internal/config ./internal/scheduler` 통과. 재시작 로그에 `dynamic_user_push`(`*/30 * * * *`)만 등록됐고 실행 코드의 cron `AddFunc` 호출도 한 곳만 남은 것을 확인했다.
- 17:55·18:01 두 작업 각각 `make test`, `git diff --check` 통과. 각 작업 후 `make restart-app`과 `/health` 정상 응답을 확인했다.
