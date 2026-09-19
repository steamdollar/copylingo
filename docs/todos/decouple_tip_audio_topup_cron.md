# Tip 및 Audio Top-up 시스템 유지보수 Cron 분리

## 배경 / 목적

기존 스케줄러(`internal/scheduler/scheduler.go`)에서는 매일 아침/저녁 정적 시각에 실행되던 `buildAndPushSessions` 내부에서 세션 발송 직후 `s.topUpTips(ctx)`가 부수 효과(side-effect)로 호출되었다.
ADR-048을 통해 사용자별 개인화 30분 주기 푸시 스케줄러(`dynamic_user_push`)가 도입되면서 세션 발송 흐름은 `sessionDispatcher`로 완전히 이관되었다.

하지만 Tip 버킷 보충(`s.topUpTips`)과 Listening Audio 사전 생성(`s.audio.EnsureListeningAudio`)은 개별 사용자의 알림 발송과 직접적 연관이 없는 **시스템 차원의 유지보수 작업(Maintenance Job)**이다.
따라서 이를 사용자 푸시와 완전히 분리하여, 독립적인 Cron 스케줄(예: 새벽 시간대 1회 실행)로 전용 Job을 등록하도록 분리한다.

## 작업 범위 및 변경할 파일

### 1. `config.yaml` & `internal/config/config.go`
스케줄 설정에 독립적인 Cron 표현식을 추가한다:
```yaml
schedule:
  dynamic_push_cron: "*/30 * * * *"
  tip_topup_cron: "0 3 * * *"      # 매일 새벽 3시 실행
  audio_pregen_cron: "0 4 * * *"   # 매일 새벽 4시 실행
```

### 2. `internal/scheduler/scheduler.go`
- `Scheduler.Start()`에 독립적인 Cron Job 등록:
  - `s.cron.AddFunc(s.cfg.Schedule.TipTopupCron, ...)` -> `s.topUpTips(ctx)`
  - `s.cron.AddFunc(s.cfg.Schedule.AudioPregenCron, ...)` -> `s.audio.EnsureListeningAudio(...)`
- 기존 `buildAndPushSessions` 내 잔존하는 `s.topUpTips` 호출 제거.

### 3. 검증 방법
- `go test -v -race ./internal/scheduler/...`
- `make test`

## 비대상 및 주의사항
- `dispatcher.go`의 사용자 푸시 파이프라인에는 일체 영향을 주지 않는다.
- LLM API 비용을 고려하여 top-up 실행 빈도는 1일 1회를 초과하지 않는다.
