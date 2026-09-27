# CopyLingo 의사결정 기록 (ADR)

## ADR-061: 여러 저장 호출의 트랜잭션 범위는 서비스에서 정한다

- 날짜: 2026-09-27
- 상태: 승인됨, Study 세션 생성부터 적용
- 배경: Study 세션과 자료 연결을 원자적으로 저장하려고 `SessionRepository.CreateStudySession`에 두 INSERT와 트랜잭션을 묶었다. 이 구조에서는 서비스가 저장 호출 사이에 다른 작업을 배치할 수 없다. Quiz 생성도 세션과 문항을 별도 호출로 저장한다.
- 결정: 여러 저장 호출을 묶어야 하는 서비스는 공유 `*sqlx.DB`를 받아 repository 계층의 `WithinTx(ctx, db, work)`를 직접 호출해 트랜잭션 범위를 정한다. 저장소 메서드는 전달받은 같은 `*sqlx.Tx`로 SQL을 실행한다. 트랜잭션 함수 값의 추가 주입이나 별도 runner 구조체·서비스 인터페이스는 두지 않는다. 먼저 Study 세션 생성에 적용하며, 기존 Quiz/Study 완료 경계는 이번 변경에 포함하지 않는다.
- 결과: 서비스에서 DB 호출의 순서와 중간 작업을 표현할 수 있고 시작·롤백·커밋 처리는 한곳에 모인다. 콜백과 저장소 계약에는 `sqlx.Tx`가 드러난다. 외부 API 호출의 효과는 PostgreSQL 롤백 대상이 아니므로 그 호출의 일관성 정책은 별도로 정해야 한다.

## ADR-062: Quiz·Study 진행 상태는 서비스를 거쳐 Redis 저장소에 직접 접근한다

- 날짜: 2026-09-27
- 상태: 승인됨
- 배경: 서비스와 Redis 저장소 사이의 `workingSetStore`가 저장 호출을 재전달하면서 오류 종류와 상태 검사를 다시 처리해 읽기 경로와 의존관계를 늘렸다. Study의 `Get`과 `CreateFromDB`는 실행 경로에서 사용하지 않는다.
- 결정: 두 서비스는 타입별 세션 저장소의 `Load/Save/Delete`를 직접 호출한다. JSON 및 버전·세션 ID 검사는 Redis 저장소의 `Load`에서 함께 처리한다. 세션 상태 미발견·손상 오류는 공통 `model.ErrSessionStoreNotFound/Corrupt`를 사용하고, 저장소 대역에 필요한 타입별 계약은 유지한다. 사용하지 않는 Study 메서드는 제거한다.
- 결과: `workingSetStore`와 그 백엔드 계약·오류 번역 설정이 사라지고 Redis 키·TTL·완료 시 DB 반영 흐름은 유지된다. 저장소 계약은 서비스 단위 테스트에서 Redis 없이 세션 상태를 주입하는 경계로 남는다.

## ADR-063: Study Redis 진행 상태에서 version 필드를 제거한다

- 날짜: 2026-09-28
- 상태: 승인됨
- 배경: `StudyActiveSessionState`의 `Version`은 현재 값 `1`만 기록하며, Redis의 Study 키에는 별도 세대 구분이 없다. JSON 해석과 요청한 session ID 일치 여부만으로 잘못된 상태를 걸러낼 수 있다.
- 결정: Study 상태의 `Version` 필드, 상수, 저장 시 대입과 version 검사를 제거한다. JSON 오류와 session ID 불일치 검사는 유지한다. Quiz 상태의 독립적인 version 검사는 변경하지 않는다.
- 결과: 기존 Redis JSON에 남은 `version` 값은 Go JSON 해석 시 무시되므로 마이그레이션 없이 읽을 수 있다. 앞으로 Study 상태 형식이 호환되지 않게 바뀌면 version 필드에 의존하지 말고 필요한 시점에 Redis 키 세대 구분이나 명시적 무효화를 추가한다.
