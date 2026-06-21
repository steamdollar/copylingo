# 오후 Study Session Push 추가

## 배경

정오 Study Session 외에 오후 4~5시 사이에 한 번 더 Study Session을 받고 싶다는 요구가 있었다.
기존 Study push는 `study_push_cron` 하나만 있었고, Scheduler가 해당 cron에서 Study Session build와 Telegram push를 함께 수행했다.

## 변경

- `schedule.afternoon_study_push_cron` 설정을 추가했다.
- 기본값과 로컬 설정은 `30 16 * * *`로 지정했다.
- Scheduler에 `afternoon_study_push` job을 추가했다.
- 기존 `buildAndPushStudySessions`를 재사용한다.

## 변경 파일

- `config.yaml`
- `internal/config/config.go`
- `internal/config/config_test.go`
- `internal/scheduler/scheduler.go`
- `internal/scheduler/scheduler_test.go`

## 검증

```bash
go test ./internal/config ./internal/scheduler -v
make test
make restart-app
```

통과.

로그에서 다음 등록을 확인했다.

```text
job=afternoon_study_push cron="30 16 * * *"
```
