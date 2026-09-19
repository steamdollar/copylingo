# ADR-048: 사용자별 맞춤 푸시 스케줄링 및 30분 폴링 엔진

- **날짜**: 2026-09-14
- **상태**: 채택됨 (Accepted)

## 맥락 (Context)
- 기존 CopyLingo 스케줄러는 `config.yaml`에 정의된 서버 전역 Cron 시각(`08:00`, `12:00`, `16:30`, `21:00` KST)에 모든 활성 사용자에게 일괄 세션을 발송하는 정적 스케줄링 방식을 사용했다.
- 사용자마다 생활 패턴, 수면 주기, 학습 가능 시각, 거주 국가의 시간대(Timezone)가 상이하므로 개인화된 시각 지정 및 슬롯별 활성화/비활성화(ON/OFF) 기능이 필수적이었다.
- 본 프로젝트의 10,000+ 유저 규모 가정(AGENTS.md §4)하에서, 빈번한 DB 폴링으로 인한 CPU/IO 병목을 방지하고, 텔레그램 API Rate Limit(30 msg/sec) 준수, 중복 발송 방지(Idempotency), 서드파티 의존성 최소화가 충족되어야 했다.

## 결정 (Decision)

### 1. 30분 이산 주기(Discrete 30-min Interval) 및 DB 스키마 최적화
- **30분 단위 제한**: 1분 단위 폴링(1일 1,440회) 대신 30분 단위(`HH:00` 또는 `HH:30`) 이산 슬롯을 채택하여 1일 48회 폴링으로 쿼리 횟수를 96.7% 절감했다.
- **DB 스키마**: `users` 테이블에 4개 독립 슬롯 컬럼과 시간대 컬럼을 추가했다:
  - `morning_study_time TIME NULL` (기본값: `08:00`)
  - `morning_quiz_time TIME NULL` (기본값: `12:00`)
  - `evening_study_time TIME NULL` (기본값: `16:30`)
  - `evening_quiz_time TIME NULL` (기본값: `21:00`)
  - `timezone VARCHAR(50) NOT NULL DEFAULT 'Asia/Seoul'`
  - 컬럼 값이 `NULL`인 경우 해당 슬롯 알림을 완전히 비활성화한다.
- **Partial Composite Indexes**: 활성화된 슬롯에 대해서만 `(timezone, <slot>_time) WHERE <slot>_time IS NOT NULL` 인덱스를 생성하여, 전체 테이블 스캔(Full Scan) 없이 O(N) 인덱스 스캔만으로 대상 사용자를 즉각 조회한다.

### 2. 표준 라이브러리 기반 Dispatcher & Concurrency 제어
- **외부 인프라(Kafka/SQS) 배제**: 프로젝트 원칙에 따라 서드파티 의존성을 배제하고 Go 표준 동시성 프리미티브(`goroutine`, `channel`, `time.Ticker`)로 Worker Pool(4개 고루틴)과 Rate Limiter(25 msg/sec)를 구축했다.
- **Telegram API Rate Limit 보호**: 텔레그램 봇 발송 한도(30 msg/sec)를 초과하지 않도록 25 msg/sec 토큰 버킷 래퍼를 통과하여 안전하게 순차/병렬 분산 발송한다.
- **N+1 쿼리 제거**: 배치 처리 대상 사용자에 대해 `CountUnfinishedBatch`(`WHERE user_id IN (?) GROUP BY user_id`) 1회의 집계 쿼리로 미완료 세션 수를 조회한다.
- **ADR-045 Backlog Cap 준수**: 미완료 세션 수가 3개 이상인 사용자는 신규 세션을 생성하지 않고, 가장 오래된 미완료 세션을 재알림한다.

### 3. Redis 분산 멱등성 락 (Idempotency Lock)
- **Key 패턴**: `copylingo:push:lock:{user_id}:{slot}:{date}` (TTL 24시간)
- **동작**: `SET NX` 성공 시에만 세션 빌드 및 텔레그램 발송을 진행하여, 서버 재시작, 다중 인스턴스 배포, 타임존 경계 중복 실행 시에도 동일 슬롯이 하루에 2회 이상 중복 발송되는 위험을 완벽히 차단했다.

### 4. 텔레그램 봇 설정 인터랙티브 UI
- `/settings` 명령어 및 메인 메뉴 "⚙️ 설정" 버튼 클릭 시 진입.
- 슬롯별 현재 설정 시각 및 꺼짐 상태 표시.
- 슬롯별 30분 단위 선택 인라인 키보드(추천 시간대 4x4 + 24시간 전체 보기 토글 + 🔕 알림 끄기 버튼).
- 시간대(Timezone) 선택 메뉴 제공(서울, 도쿄, 뉴욕, LA, 런던, UTC 등).

## 결과 / 트레이드오프 (Consequences & Tradeoffs)
- **장점 (Pros)**:
  - 1만 명 이상의 사용자 및 다중 시간대 환경에서도 DB 부하를 최소화하면서 개인화된 알림을 안정적으로 발송.
  - 외부 큐 시스템 없이도 프로세스 안전성, Rate Limiting, 멱등성이 보장됨.
  - 모바일 Telegram 환경에 최적화된 직관적이고 빠른 설정 UX 제공.
- **단점 및 제약 (Cons & Limitations)**:
  - 30분 단위만 지원하므로 `08:15` 등 임의의 분 단위 설정은 불가.
  - 시스템 유지보수성 작업(Tip 버킷 충전, Listening Audio 사전 생성)은 사용자 푸시와 분리되어 독립 Cron(새벽 시간대)으로 이관 필요 (Case C TODO 분리).
