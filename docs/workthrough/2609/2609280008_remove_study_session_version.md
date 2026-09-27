# Study Redis 진행 상태의 version 필드 제거

## 결정

Study Redis 상태는 JSON 유효성과 요청 session ID 일치 여부로 검증한다. 현재 한 값만 사용하는 `Version` 필드와 그 대입·비교를 제거했다. Quiz의 version 검사는 유지한다. 근거와 호환성은 [ADR-063](../../adr/ADR_from_61_to_80.md#adr-063-study-redis-진행-상태에서-version-필드를-제거한다)에 기록했다.

## 변경

- `internal/model/study_active_session.go`, `internal/repository/study_active_session_repo.go`, `internal/service/study_active_session.go`: Study 상태 필드·상수·초기화를 제거했다.
- `internal/redisstore/sessions.go`, `session_store.go`: Study validator는 session ID만 확인하고 공통 Load 주석을 설정된 validator 기준으로 정리했다.
- 관련 Redis·서비스·봇 테스트 초기화에서 Study version 값을 제거했다. Redis 키·TTL, JSON 오류 및 session ID 불일치 처리는 유지했다.
- `docs/review_flows/2606062354_study_session_review_flow.md`: 제거된 version 재구성 항목을 갱신했다.

## 검증

- 수정한 Go 파일에 `goparams`를 적용했다.
- `make test`: 통과.
- `make restart-app`: 성공, `http://localhost:8080/health` 준비 확인.
