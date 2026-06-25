# 손글씨 오답 이미지 저장 로깅

## 배경

handwriting 문항에서 맞게 쓴 답안이 오답 처리되는 사례를 디버깅하기 위해, LLM에 전달된 서버 복원 PNG를 사후 확인할 수 있어야 한다.

이번 변경은 채점 로직이나 prompt는 건드리지 않고, handwriting 오답일 때만 복원 이미지를 로컬 로그 디렉터리에 저장하는 좁은 관측성 보강이다.

## 변경 파일

- `internal/service/handwriting.go`
  - handwriting 제출이 오답으로 채점되면 `logs/images/<session_question_id>.png`에 렌더링된 PNG를 저장한다.
  - 저장 성공/실패를 structured log로 남긴다.
  - 이미지 저장 실패는 사용자 응답을 실패시키지 않는다.
- `internal/service/handwriting_test.go`
  - 오답 handwriting 제출 시 `session_question_id` 파일명으로 렌더링 이미지가 저장되는 회귀 테스트를 추가했다.
- `STATUS.md`
  - 최근 완료 항목에 이번 side task를 추가했다.

## 결정 사항

- 파일명은 `question_id`가 아니라 `session_questions.id`를 사용한다. 같은 question이 여러 session/occurrence에 등장해도 디버그 이미지가 덮이지 않게 하기 위함이다.
- 저장 경로는 요청대로 `logs/images`를 사용한다. `logs/`는 이미 `.gitignore`/`.dockerignore` 대상이다.
- 오답 이미지 저장은 best-effort로 처리한다. 디스크 권한/용량 문제로 저장에 실패해도 채점 결과 반환은 유지한다.

## 검증 결과

```bash
go test ./internal/service -run 'TestSubmitAnswer|TestGradeHandwriting'
make test
make restart-app
curl -fsS http://localhost:8080/health
```

결과: 모두 통과.

Health 응답:

```json
{"status":"healthy","time":"2026-06-23T20:03:07+09:00"}
```
