# Study 복습 부족분 신규 단어 보충

## 배경과 결정

- 운영 DB 조회에서 2026-09-18 Study가 아침 19/20개, 저녁 16/24개로 생성된 것을 확인했다. 신규량은 아침 10개·저녁 5개로 유지됐지만 due 복습이 각각 9개·11개여서 목표에 미달했다.
- 조회 시점에 Study 범위 내 신규 단어 746개가 남아 있었다. 사용자는 기존 due 보충 이후에도 부족하면 현재 레벨 우선 신규 단어로 채우는 방안을 승인했다.
- [ADR-049](../../adr/ADR_from_41_to_60.md#adr-049-study-복습-후보-부족분은-신규-단어로-보충한다)에 결정과 신규 학습 부담 증가의 절충을 기록했다. 별도 설정·스키마·앱 측 후보 로딩 없이 기존 SQL 안에서 처리한다.

## 변경 파일과 동작

- `internal/repository/material_repo.go`: 기존 유형별 선정 → 같은 유형 due 보충 → Vocabulary/Grammar due 보충 이후, 남은 수량만큼 미선택 신규 Vocabulary를 추가한다. 기존 신규 bucket 순위로 현재 레벨·난이도·무작위 우선순위를 유지한다.
- `internal/model/study_session.go`: 신규 Vocabulary 보충이 가능한 plan 계약으로 주석을 정리했다.
- `internal/repository/material_repo_test.go`: 실제 PostgreSQL 임시 테이블로 아침 20개·저녁 24개 보충, due 우선, 신규 문법·독해 제한, 현재 레벨 우선, 미래 복습·미완료 세션 제외, 중복 방지, 인접 레벨까지 소진 후 짧은 세션 반환을 검증한다.
- `docs/adr/ADR_from_41_to_60.md`, `docs/adr/ADR-047_study_morning_evening_mix.md`: ADR-049 추가 및 이전 결정의 변경 지점을 연결했다.
- `STATUS.md`: 완료 항목을 추가했다.

## 적용 범위

- 아침/저녁 Study와 같은 repository를 사용하는 수동 `/study n`에 적용한다.
- 신규 Grammar·Reading의 할당과 Reading 총량 제한을 유지한다. 미래 복습을 당기지 않고 미완료 Study에 편성된 Material도 재사용하지 않는다.
- due·신규 Vocabulary 후보까지 부족하면 목표보다 짧아질 수 있다. 복습이 적은 날 신규 단어 수와 학습 시간이 늘어날 수 있다.
- 기존 세션과 학습 이력을 재작성하지 않으며, 다음에 생성되는 세션부터 적용한다.

## 검증과 런타임 반영

- `gofmt`, `git diff --check` 통과.
- `COPYLINGO_TEST_DATABASE_URL`을 설정한 `make test` 전체 통과: 테스트가 있는 16개 package, `TestGetForStudySessionPostgres` 및 신규 하위 시나리오 모두 통과.
- PostgreSQL 테스트는 transaction 내부 임시 테이블만 사용하고 rollback한다. 운영 세션을 생성하거나 사용자에게 메시지를 보내는 검증은 수행하지 않았다.
- `make restart-app` 성공. target의 `http://localhost:8080/health` 확인 통과.
