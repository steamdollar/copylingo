# Quiz·Study 세션 진행 상태 저장 경로 단순화

## 결정

Redis 진행 상태에 접근할 때 서비스와 저장소 사이의 `workingSetStore`를 제거한다. 서비스는 타입별 저장소의 `Load/Save/Delete`를 직접 호출한다. 구조적·의미적으로 잘못된 Redis 상태는 저장소에서 삭제하고 공통 손상 오류를 반환한다. 근거는 [ADR-062](../../adr/ADR_from_61_to_80.md#adr-062-quizstudy-진행-상태는-서비스를-거쳐-redis-저장소에-직접-접근한다)에 기록했다.

## 변경

- `internal/service/working_set.go` 삭제: 공통 저장소 포장, 추가 백엔드 인터페이스 및 서비스별 오류 번역을 제거했다.
- `internal/service/study_active_session.go`, `quiz_active_session.go`: 기존 타입별 저장소 계약으로 Redis 상태를 직접 읽고 저장한다. 실행 경로에서 사용하지 않는 Study `Get`과 `CreateFromDB`를 제거했다.
- `internal/redisstore/session_store.go`, `sessions.go`: JSON 해석과 버전·세션 ID 검사를 한 `Load`에서 수행하고 손상된 키를 삭제한다. 기존 키 이름과 24시간 TTL은 유지한다.
- `internal/model/session_store.go`: 사용하지 않는 저장소 의존성 오류를 제거하고 미발견·손상 오류는 공통으로 사용한다.
- 관련 서비스·봇·Redis 저장소 테스트: 미사용 메서드 호출을 현재 경로로 옮기고, 손상 상태 삭제 검증을 실제 Redis 저장소 구현 테스트로 이동했다.

## 남긴 계약

`StudySessionStore`와 `QuizSessionStore`는 서비스 테스트가 Redis 없이 상태를 주입할 때 사용하는 최소 `Load/Save/Delete` 계약이므로 유지했다. 소유권 검사와 완료 시 DB 반영도 기존 서비스·저장소가 담당한다. `GetOwned`의 조회 시 TTL 갱신은 변경하지 않았다.

## 검증

- 대상 패키지 테스트 및 `make test`: 통과.
- `make restart-app`: 재시작 후 `/health` 준비 상태 확인.
