# 세션별 문제 보기(Options) 결정론적 셔플링 구현

## 1. 배경 및 목적
- **문제점**:
  - `questions` 테이블의 `options` 컬럼에 보기가 고정 배열로 저장되어 있어, 객관식 57.3%(949/1,657건), 청해 70.0%(91/130건)에서 1번 보기(`options[0]`)가 정답으로 심하게 편향되어 있었음.
  - 동일 question row가 출제될 때마다(특히 SRS 복습) 정답 보기가 항상 동일한 위치에 노출되어 유저가 위치(sequence)만으로 정답을 외우는 학습 결함 존재.
- **해결 방안 (Option A)**:
  - 세션 생성(`ActiveSessionService.CreateFromDB`) 시점에 `sessionID:questionID` 기반 FNV-1a 해시를 Seed로 사용하는 **결정론적 셔플(Deterministic Shuffle)** 적용.
  - 동일 세션 내에서는 서버 재시작 및 캐시 재적재가 발생해도 화면의 버튼 인덱스와 서버 상태가 100% 일치(Idempotency 보장).
  - 다른 세션(새로운 `sessionID`)으로 출제 시에는 Seed가 변경되어 보기 순서가 동적으로 재배치됨.

---

## 2. 변경 파일 및 세부 내용

### 1) [internal/model/question.go](../../internal/model/question.go)
- `(q *Question) ShuffleOptions(sessionID int) error` 메서드 추가:
  - `len(q.Options) <= 1` 또는 옵션이 비어있는 경우 No-op 처리 (주관식/빈칸 채우기 안전).
  - `fnv.New64a()`로 `fmt.Sprintf("%d:%d", sessionID, q.ID)` 해시 생성 후 `rand.New(rand.NewSource(...))`로 셔플.
  - 셔플된 배열을 다시 JSON 인코딩하여 `q.Options`에 반영.

### 2) [internal/service/active_session.go](../../internal/service/active_session.go)
- `CreateFromDB(ctx, sessionID)`에서 DB 조회 후 Redis 적재 전 `state.Items`의 각 문항에 대해 `state.Items[i].Question.ShuffleOptions(sessionID)` 실행.
- Redis 캐시 미스로 복구(`Get` $\rightarrow$ `CreateFromDB`) 시에도 동일한 Seed로 동일 순서 복원 보장.

### 3) 단위 테스트 추가
- **[internal/model/question_test.go](../../internal/model/question_test.go)**: `TestQuestion_ShuffleOptions`
  - 동일 sessionID/questionID에 대한 결정론적 일관성 검증.
  - 옵션 원소 100% 보존 검증.
  - 서로 다른 sessionID에 대한 순열 다양성 검증.
  - 빈 배열/단일 원소/nil/invalid JSON 엣지 케이스 검증.
- **[internal/service/active_session_test.go](../../internal/service/active_session_test.go)**: `TestActiveSessionCreateFromDB_ShufflesQuestionOptions`
  - 세션 생성 시 셔플 반영 및 동일 세션 재적재 시 순서 보장, 다른 세션 순서 변경 검증.

---

## 3. 검증 결과
- `go test ./internal/model/... -v` 통과
- `go test ./internal/service/... -v` 통과
- `go test ./... -v` 전체 프로젝트 테스트 100% PASS
