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

---

## ADR-022: Study Material은 Question과 분리된 SSOT로 관리

- **날짜**: 2026-06-02
- **상태**: 채택됨 (Task 1 완료, Session 연결은 후속 Task)
- **맥락**:
  - 기존 앱은 `questions`를 직접 Seed하고 출제하므로 사용자가 Quiz 전에 학습 개념을 익히는 흐름이 없다.
  - 같은 단어도 객관식, 입력, 손글씨 Question으로 중복 표현된다.
  - `contents`는 NHK 등 외부 수집 원문이며, 가나와 단어처럼 코드로 Seed하는 학습 단위와 lifecycle이 다르다.
- **결정**:
  - `materials` 테이블을 Study Module의 학습 단위 SSOT로 추가한다.
  - `materials.content_id`는 nullable FK로 둔다. 코드 Seeder 기반 Material은 외부 원문 없이 존재할 수 있다.
  - 카드 형태 차이는 `payload JSONB`로 수용하고, 공통 검색 조건은 `category`, `language`, `proficiency_level`, `difficulty` 컬럼으로 유지한다.
  - `material_key`는 `{language}:{domain}:{stable_slug}` 형식의 Business Key이며 UNIQUE Constraint로 Idempotent Upsert를 보장한다.
  - Kana Slug는 Romaji 중복을 피하기 위해 Unicode code point를 사용한다. 예: `ja:kana:u3042`.
  - Vocabulary Slug는 Level을 제외한 안정 ID를 사용한다. 예: `ja:vocab:word_024`.
  - Material Seed는 `cmd/ja/material_seeder`가 독립적으로 수행한다. 기존 Kana/Vocab Question Seeder는 Material을 변경하지 않는다.
  - 초기 Material Seed 범위는 Vocabulary로 제한한다. Kana, Grammar, Sentence Material은 Study UX 도입 순서에 맞춰 후속 추가한다.
  - 기존 Quiz Question과 SRS 구조는 Task 1에서 변경하지 않는다.
- **장점**:
  - Study 개념과 Quiz 표현을 분리하여 향후 하나의 Material에 여러 Question 유형을 연결할 수 있다.
  - Seeder 재실행으로 Material이 중복되지 않는다.
  - `contents` 수집기 상태와 무관하게 기초 학습 Material을 운영할 수 있다.
- **단점 / 트레이드오프**:
  - Study Session 연결 전까지 `materials`는 Quiz 흐름에서 직접 사용되지 않는다.
  - `payload JSONB` 구조는 Category별 Consumer가 명시적으로 해석해야 한다.
  - 초기 Vocabulary Catalog는 기존 Question Seeder 데이터와 일부 중복된다. 두 Seeder의 lifecycle 격리를 우선하고, 중복 변경 비용이 커질 때 공용 Catalog를 재검토한다.
- **대안**:
  - `contents` 재사용: 외부 원문과 기초 학습 개념의 lifecycle이 섞여 기각.
  - `questions` 재사용: Quiz 표현 중복 때문에 Study 진도 SSOT로 부적합하여 기각.
  - `material_key` 없이 SERIAL PK만 사용: Seeder Idempotency를 보장하기 어려워 기각.
