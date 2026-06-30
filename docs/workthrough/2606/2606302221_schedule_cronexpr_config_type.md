# ScheduleConfig cron 필드 CronExpr 커스텀 타입 전환

## 배경

`internal/config/config.go` 의 `ScheduleConfig` 는 7개 cron 필드를 모두 plain `string` 으로 들고 있었다.

문제:

- config 타입만 봐서는 cron expression 임이 드러나지 않았다.
- 잘못된 cron expression 이 `Load()` 단계에서 걸러지지 않고 scheduler 등록 시점(`cron.AddFunc`)까지 내려갔다.
- cron 오타는 조용한 기능 중단으로 이어지므로 app startup 초기 fail-fast 가 운영 안정성에 유리하다.

`docs/todos/schedule_cronexpr_config_type.md` 의 **Option A** 로 확정되어 진행했다.

## 결정 사항

- `CronExpr string` 커스텀 타입 도입 (`String()`, `IsZero()`, `Validate(name)`).
- `Validate` 는 `cron.ParseStandard` 로 검증하되, **non-empty 값만** 검증한다.
  빈 값은 통과시켜 기존 backward compatibility 를 유지한다
  (`content_collect_cron` 은 빈 값이면 scheduler 가 job 등록을 skip 하는 경로가 있음).
- `ScheduleConfig` 7개 필드를 모두 `CronExpr` 로 전환.
- `Config.validate()` 에서 logging validation 이후 `c.Schedule.validate()` 를 호출해 fail-fast.
- scheduler 호출부는 `AddFunc` 인자·로그 `cron` 값을 `.String()` 으로, content-collect 가드를 `.IsZero()` 로 명시 변환.

### TODO 문서 누락 보정

TODO 문서의 Before/After 스니펫과 `validate()` checks 슬라이스가 `AfternoonStudyPushCron`
(`afternoon_study_push_cron`) 을 누락했다. 실제 코드엔 이 필드가 있어 총 **7개** cron 필드다.
이 필드도 동일하게 `CronExpr` 전환 + validate + scheduler `.String()` 적용에 포함했다.

### 건드리지 않은 것

- `config.yaml` key 이름·값, env key (`COPYLINGO_SCHEDULE_*`)
- scheduler job 종류·실행 순서
- `MorningBuildCron`, `EveningBuildCron` 사용 정책
- DB schema / migration, ADR 문서

## 변경 파일

- `internal/config/config.go` — `CronExpr` 타입 + `ScheduleConfig.validate()` 추가, 필드 7개 타입 전환, `Config.validate()` 연결, `robfig/cron/v3` import
- `internal/config/config_test.go` — 기존 schedule 테스트를 `.String()` 비교로 갱신, invalid cron / empty cron / Validate 단위 테스트 추가
- `internal/scheduler/scheduler.go` — `AddFunc` 5곳 + content-collect 가드 + 로그 `cron` 값을 `.String()`/`.IsZero()` 로 변환

## 검증

```bash
go build ./...                                  # 통과
go test ./internal/config ./internal/scheduler  # ok / ok
go vet ./internal/config ./internal/scheduler   # 통과
gofmt -l ...                                     # clean
```

추가한 테스트:

- `TestLoadRejectsInvalidScheduleCron` — invalid cron 이 `Load()` 단계에서 에러 반환
- `TestCronExprAllowsEmpty` — 빈/공백 cron 은 `Validate()` 통과, `IsZero()` true
- `TestCronExprValidateRejectsInvalid` — invalid cron 은 `Validate()` 에서 에러

기존 `scheduler_test.go` 의 struct literal(`MorningPushCron: "0 8 * * *"` 등)은 untyped string
constant 라 `CronExpr` 필드에 그대로 할당되어 무변경 통과.
