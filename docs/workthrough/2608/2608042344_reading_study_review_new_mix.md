# Reading Study 복습·신규 혼합 편성

## 배경

ADR-036의 Study Session당 Reading 1개 제한을 조정해, 새 지문 진도와 SRS 복습을 같은 세션에서 병행하도록 했다. Reading을 모두 학습한 뒤에는 신규 bucket 대신 due review를 최대 2개 제공한다.

## 결정

- unseen Reading이 남아 있으면 due review 최대 1개 + unseen new 최대 1개를 선택한다.
- unseen Reading이 없으면 due review를 최대 2개 선택한다.
- review는 `next_review_at <= NOW()`인 due 항목만 후보로 둔다. 아직 due가 아닌 review는 강제로 당기지 않는다.
- Reading bucket끼리는 부족분을 빌리지 않는다. Reading이 2개 미만이면 전체 `LIMIT`의 남는 자리는 Vocabulary/Grammar가 채운다.
- 기존 `pending`/`in_progress` Study Session 중복 제외와 기타 category의 rank/order를 유지한다.
- 최종 정렬에서 Reading은 due/new bucket rank를 selection rank로 사용하고, 같은 rank에서는 due를 new보다 먼저 둔다. 다른 category는 기존 category rank interleave를 유지한다.
- Daily Quiz의 Reading 최대 1개 제한은 변경하지 않는다.

## 변경 파일

- `internal/repository/material_repo.go`
  - language/level/category 후보 pool과 Reading unseen inventory를 분리했다.
  - category 전체 rank는 기존 정렬을 유지하고, due/new bucket rank를 추가해 Reading quota만 적용했다.
  - Reading 최종 정렬에도 bucket rank를 사용해 due가 여러 개여도 선택된 new가 global `LIMIT` 밖으로 밀리는 starvation을 막았다.
- `internal/repository/material_repo_test.go`
  - due+new 공존, all-seen, due 없음+new, non-reading/limit, 다수 due에 의한 new starvation, active session 중복 제외 SQL 계약 테스트를 추가했다.
- `docs/adr/ADR_from_21_to_40.md`
  - ADR-040으로 편성 정책과 트레이드오프를 기록했다.
- `STATUS.md`
  - 최근 완료 항목을 추가했다.

## 검증

- `gofmt -w internal/repository/material_repo.go internal/repository/material_repo_test.go`: 통과
- `go test ./internal/repository -run 'TestStudySession' -v`: 통과
- local PostgreSQL 기존 owner data에 변경 query를 read-only로 실행: eligible due Reading 4개·unseen Reading 27개인 상태에서 `LIMIT 5` 결과에 due Reading 1개와 unseen Reading 1개가 각각 2·3번째로 포함되고, 나머지 자리는 Vocabulary가 채우는 것을 확인
- `make test`: 전체 통과
- `make restart-app`: 성공 (`[copylingo] app is ready: http://localhost:8080/health`)
- `curl -fsS http://localhost:8080/health`: `{"status":"healthy", ...}` 확인

## 남은 위험

- repository test는 이 프로젝트의 기존 방식대로 SQL 문자열 계약을 검증한다. 실제 PostgreSQL query는 local catalog로 별도 smoke했지만, fixture 기반 DB integration test는 없다.
- unseen 여부를 active-session 제외 전 catalog 기준으로 보므로 unseen Reading이 이미 다른 active Study Session에 있으면 현재 세션의 Reading은 2개 미만일 수 있다. 이는 bucket 부족 시 review로 보충하지 않는 결정에 따른 동작이다.
