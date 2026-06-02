# CopyLingo 의사결정 기록 (ADR)

## ADR-021: Application Log는 Context 기반 JSONL Structured Logging으로 기록

- **날짜**: 2026-06-01
- **상태**: 채택됨
- **맥락**:
  - 기존 로그는 Standard Library `log.Printf` 문자열이 여러 경계 계층에 흩어져 있다.
  - Telegram Update, Mini App HTTP 요청, Scheduler job에서 발생한 하위 로그를 하나의 상호작용 단위로 추적하기 어렵다.
  - 현재 운영 단계에서는 중앙 로그 수집기보다 로컬에서 직접 조회 가능한 일별 파일이 우선 필요하다.
- **결정**:
  - Standard Library `log/slog` JSON Handler를 사용한다. 외부 logger Dependency는 추가하지 않는다.
  - 로그는 stdout과 `logs/copylingo-YYYY-MM-DD.jsonl`에 동시에 기록한다.
  - 파일명과 JSON timestamp는 기본적으로 `Asia/Seoul` 기준이며, 30일이 지난 규약 파일은 자동 삭제한다.
  - HTTP 요청, Telegram Update, Scheduler job 진입점에서 `interaction_id`를 생성하고 `context.Context`로 하위 레이어에 전달한다.
  - 파일 sink 장애는 stderr 경고 후 stdout-only로 degrade한다. Application Log 보존 실패 때문에 서비스 전체를 중단하지 않는다.
  - 숫자 식별자는 기록할 수 있지만 token, Telegram `init_data`, 사용자 답안 원문, stroke 좌표는 기록하지 않는다.
  - 파일 로그는 장애 분석용이며 DB 상태나 Audit Log의 SSOT로 사용하지 않는다.
- **장점**:
  - 외부 Dependency 없이 건별 correlation과 JSON 기반 조회가 가능하다.
  - stdout을 유지하므로 Docker logging driver 또는 향후 중앙 collector로 전환하기 쉽다.
  - 파일 sink 장애와 서비스 가용성을 분리한다.
- **단점 / 트레이드오프**:
  - 단일 서버 파일은 수평 확장 환경에서 통합 조회가 어렵다.
  - 파일 cleanup과 rotation 책임이 애플리케이션에 추가된다.
  - 일부 기존 `log.Printf`는 점진 전환 기간 동안 `legacy.log` event로 남는다.
- **대안**:
  - Uber `zap`: 고빈도 logging 성능은 우수하지만 현재 로그량에서 외부 Dependency 비용 대비 실익이 작아 기각.
  - stdout-only + Docker logging driver: 운영은 단순하지만 로컬에서 일별 파일을 직접 조회하려는 요구를 충족하지 못해 기각.
  - DB Audit Log: 상태 변경 이력 보존에는 적합하지만 이번 요구는 장애 분석용 Application Log이므로 별도 범위로 분리.
