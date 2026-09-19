# 사용자별 맞춤 푸시 스케줄링 및 30분 폴링 엔진 (ADR-048)

## 요청과 결정

- **요청**: 고정된 전역 Cron 기반 세션 발송을 유저별 맞춤형 시각 지정 및 슬롯 활성화/비활성화(ON/OFF)가 가능한 동적 스케줄링 시스템으로 전면 개편.
- **아키텍처 결정 (BIG CHANGE 4단계 검토 완료)**:
  - 10,000+ 유저 다중 시간대(Timezone) 가정 하에 1분 폴링 대신 **30분 이산 슬롯(`HH:00`, `HH:30`)** 채택 (1일 48회 폴링으로 DB 부하 96.7% 절감).
  - 외부 큐(Kafka/SQS) 도입 없이 Go 표준 동시성 프리미티브(`goroutine`, `channel`, `time.Ticker`)로 Worker Pool(4개) 및 Rate Limiter(25 msg/s) 구현.
  - Redis 분산 멱등성 락(`copylingo:push:lock:{user_id}:{slot}:{date}`, TTL 24h)으로 중복 발송 차단.
  - 4개 독립 슬롯(`morning_study`, `morning_quiz`, `evening_study`, `evening_quiz`, `TIME NULL`), 다중 타임존 partial composite index 생성.
  - 텔레그램 봇 `/settings` 및 메인 메뉴 설정 UI 구현.
  - 세션 발송과 무관한 Tip/Audio 유지보수는 Case C TODO로 분리.
- 상세: [ADR-048](../../adr/ADR-048_personalized_push_scheduling.md), 계획서: `docs/plan_dynamic_user_scheduling.md`.

## 변경 범위

1. **DB 마이그레이션**:
   - `migrations/002_user_session_schedules.sql`: `users` 테이블에 4개 슬롯 컬럼(`TIME NULL`) 및 `timezone` 추가. 기존 레거시 컬럼(`morning_session_time`, `evening_session_time`)으로부터 백필 및 `(timezone, <slot>_time)` partial composite index 생성.
   - `schema.dbml`: DBML 문서 최신화.
2. **도메인 모델 & 리포지토리**:
   - `internal/model/user.go`: `SessionSlot` 타입 및 상수(`morning_study`, `morning_quiz`, `evening_study`, `evening_quiz`), `User` 구조체 필드 포인터(`*string`) 확장.
   - `internal/repository/user_repo.go`: `GetActiveTimezones`, `GetUsersBySlot`, `UpdateSlotTime`, `UpdateTimezone` 추가 및 인덱스 활용 쿼리 최적화.
   - `internal/repository/session_repo.go`: `CountUnfinishedBatch` 추가 (N+1 쿼리 제거, `WHERE user_id IN (?) GROUP BY user_id`).
3. **서비스 레이어**:
   - `internal/service/user.go`: `IsValid30MinTime` 검증 함수, 슬롯 변경 및 IANA 타임존 유효성 검증 로직 구현.
   - `internal/service/session_query.go`: `CountUnfinishedBatch` 노출.
4. **스케줄러 & 디스패처**:
   - `internal/scheduler/dispatcher.go`: `sessionDispatcher` 구현 (4 Worker Pool, 25 msg/s Rate Limiter, Redis 멱등성 락, ADR-045 Backlog Cap 검사, Study/Quiz 통합 발송).
   - `internal/scheduler/scheduler.go`: 30분 주기 `tick()` 등록 및 레거시 cron과 호환되는 듀얼 실행 구조 지원.
   - `cmd/server/server.go`, `cmd/server/main.go`: `rdb redis.Cmdable` 주입.
   - `internal/config/config.go`, `config.yaml`: `dynamic_push_cron: "*/30 * * * *"` 추가.
5. **텔레그램 봇 UI**:
   - `internal/config/constants.go`: `PrefixSettings`, `ActionSettingsView`, `ActionSettingsTimezone`, `CommandSettings` 추가.
   - `internal/bot/settings.go`: 설정 개요 화면, 슬롯별 30분 단위 시간 선택 인라인 키보드(추천 시간 4x4 + 24시간 전체 보기 토글 + 끄기 버튼), 타임존 변경 화면 구현.
   - `internal/bot/handler.go`: `/settings` 명령어 라우팅, `menu:settings` 및 `settings:*` 콜백 처리 연동, 도움말 갱신.
6. **테스트**:
   - `internal/repository/user_repo_test.go`: 타임존 및 슬롯별 사용자 조회 단위 테스트.
   - `internal/scheduler/dispatcher_test.go`: 동시성 Worker Pool, Rate Limiter, Redis 멱등성 락, Backlog Cap 테스트 (`-race`).
   - `internal/bot/settings_test.go`: 설정 명령어, 슬롯 선택, 시각 변경, OFF 처리, 타임존 변경, 유효하지 않은 입력 예외 처리 테스트.

## 검증 결과

- `make test`: 전체 패키지 단위/통합 테스트 100% 통과 (Race Detector 및 PostgreSQL 임시 테이블 포함).
- `go test -v -race ./internal/scheduler/...`: 통과.
- `go test -v -race ./internal/bot/...`: 통과.
- `go test -v -race ./internal/service/...`: 통과.

## 후속 작업 (Case C TODO)

- `docs/todos/decouple_tip_audio_topup_cron.md`: 사용자 세션 푸시와 분리된 Tip 버킷 충전 및 청해 음성 사전 생성 독립 Cron 분리.
