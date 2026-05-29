# Redis Active Session State 구현

## 배경

`showQuestion`, text answer, handwriting submit 흐름에서 `sessions`, `session_questions`, `questions`를 반복 조회했다. 손글씨 제출은 `session_id`, `question_id`를 이미 알고 있는데도 service와 grader가 다시 DB lookup을 수행했다.

이번 변경은 read-through cache가 아니라 ADR-013 방향의 active session working state를 구현한다. 진행 중 progress는 Redis가 담당하고, DB는 세션 종료 시점에 최종 flush되는 저장소로 둔다.

## 변경 사항

- `model.ActiveSessionState`, `model.ActiveSessionQuestion` 추가
  - session metadata, ordered session question, question copy, progress, current index, answered count를 한 Redis working set으로 저장한다.
- Redis key 추가
  - `session:%d:working_set`
- `repository.ActiveSessionRepository` 추가
  - session + session_questions + questions JOIN load
  - 세션 종료 flush transaction
- `service.ActiveSessionService` 추가
  - `CreateFromDB`, `Get`, `RecordAnswer`, `SetCurrentIndex`, `Flush`, `Delete`
  - Redis state missing/corrupt 시 DB fallback 없이 error 반환
- `GraderService` 변경
  - question DB lookup과 per-answer DB write 제거
  - Redis state의 question으로 채점하고 `RecordAnswer`로 progress/SRS/stat working copy 갱신
  - `CompleteSession`에서 Redis state를 DB에 flush 후 streak 갱신 및 Redis state 삭제
- `HandwritingService` 변경
  - session ownership, question membership, already answered, question type 검증을 Redis state 기준으로 수행
  - grader에 cached question을 전달해 중복 lookup 제거
- `SessionFlow`, restart recovery, Mini App message refresh 변경
  - 문제 표시/답변 여부/다음 문제 계산/결과 요약을 Redis active state 기준으로 수행

## 결정 사항

- Redis state는 `cache`가 아니라 진행 중 세션의 working store다.
- Redis state가 없거나 깨진 경우 stale DB로 복구하지 않는다.
- answer hot path에서 DB write는 하지 않는다.
- 완료 시점 flush는 DB transaction으로 수행한다.
- `questions` stats는 flush 시 delta increment로 반영하고, SRS field는 Redis working copy의 최종 값을 반영한다.

## 검증

```bash
go test ./...
make test
```

검증 결과는 작업 종료 메시지에 기록한다.

결과:

- `go test ./...` PASS
- `make test` PASS

## 후속 리스크

- 현재 `RecordAnswer`는 Redis JSON state의 read-modify-write 방식이다. 단일 app instance 운영에서는 충분하지만, 다중 app instance에서 같은 question에 동시 submit이 들어오는 경우 Redis `WATCH` 또는 Lua CAS로 atomic update를 보강할 수 있다.
- `questions` 테이블에 SRS state가 전역으로 저장되는 기존 설계는 유지했다. 사용자별 SRS가 필요하면 별도 ADR과 migration이 필요하다.
