# SRS 첫 복습 간격 1일 → 3일 상향 (study material + quiz question)

## 배경

"스터디/퀴즈 세션에서 본 게 계속 또 나온다"는 체감 보고. 세션 빌드 메커니즘을 점검한 결과 버그가 아니라 **SRS 정책이 하루 2회 push 환경에서 과하게 공격적**인 것이 원인으로 확인됐다.

### 진단 (DB 실측)

- 처음 의심한 "미완료 세션으로 `next_review_at`이 안 밀려 반복" 가설은 **기각**: `user_material_progress.next_review_at IS NULL` 0건, 미완료(pending/in_progress) 세션 0건 — flush 정상.
- 진짜 원인: SM-2 첫 복습 간격이 **1일**이라 학습한 material이 다음날 곧장 due로 재등장. study material 분포상 `repetitions=1, interval=1`이 49개(그중 28개 즉시 due), 하루 2회 push(`study_push` + `afternoon_study_push`)와 due-first 정렬·무작위성 부재가 겹쳐 동일 묶음이 반복.
- 증거: 세션 #27(material 267~276)이 다음날 세션 #36에 동일 묶음으로 재등장.
- quiz의 미학습 fallback 우려도 점검: 출제 113건 중 studied 111 / 미학습 2건으로, 후보 풀(studied 신규 264개)이 충분해 fallback은 거의 안 탐. study 신규 소화가 빨라지면 fallback 의존은 오히려 감소하므로 본 변경과 상충 없음.

## 변경 사항

첫 복습 간격(`repetitions = 0` 분기)을 1일 → 3일로 상향. 이후 시퀀스는 3→6→ease 로 자연 증가. 오답 reset(1일)은 SRS 정상 동작이라 그대로 유지.

- `internal/service/srs.go`
  - `updateSchedule` `case 0: q.IntervalDays = 1` → `3`
- `internal/service/srs_test.go`
  - `CorrectFirstRepetition` 기대값 `expectInt 1` → `3`
- `internal/repository/study_active_session_repo.go` (`flushUserMaterialProgress`)
  - INSERT 신규: `interval_days 1 → 3`, `next_review_at NOW()+1day → NOW()+3day`
  - ON CONFLICT `interval_days`/`next_review_at` 의 `repetitions = 0 THEN 1` → `THEN 3` (2곳)

## 결정

- 적용 범위: study material SRS + quiz question SRS **둘 다** (동일 1일 구조라 일관성 확보).
- 간격 값: **3일** (3→6→15 시퀀스 자연스러움, 복습 효과 유지하며 매일 재등장 차단).
- ADR 미기록: 수치 policy 튜닝으로 판단 (사용자 합의).

## 후속 (미적용, 선택)

- 정렬 무작위성 부재(스터디 `m.id ASC`로 끝나 매번 동일 순서)는 별도 항목(처음 피드백의 C안). 본 변경으로 매일 재등장은 끊기므로 체감 우선순위 낮아짐.
- 기존 데이터: `repetitions=1, interval=1`인 49개는 다음 학습 시 6일로 정상화되나, 즉시 due인 항목은 한 번 더 노출될 수 있음. 보정 UPDATE는 사용자 판단 대기.

## 검증

- `make test` 전체 통과 (exit=0, FAIL 0).
- `make restart-app` → `http://localhost:8080/health` healthy 확인.
