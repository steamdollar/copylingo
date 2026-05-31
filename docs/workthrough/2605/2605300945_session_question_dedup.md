# 동일 세션 중복 문항 출제 수정

## 배경

진행 중인 학습 세션에서 이미 풀었던 문제가 다시 출제되고, 답안을 입력하면
`이미 답변한 문제입니다.`가 표시되는 문제가 발생했다.

tmux app pane 로그와 로컬 DB, Redis working set을 확인한 결과:

- `session_id=131`의 `question_id=447`이 `question_order={6,13}`으로 두 번 포함됐다.
- Redis working set의 `current_index=13`에서 두 번째 문항은 미답변 상태였다.
- 기존 lookup은 `question_id`로 첫 번째 row를 찾아 이미 답변된 `question_order=6`을 반환했다.

## 원인

`SessionBuilder`의 Random Slot Relay는 카테고리별 조회 후 마지막 fallback 조회를 수행하지만,
앞 단계에서 이미 고른 question ID를 제외하지 않았다. fallback이 같은 문항을 다시 반환할 수 있었다.

또한 callback 규약은 `q:{session_id}:{question_id}:{option_idx}` 형태이므로,
active session lookup이 단순히 첫 번째 question ID를 반환하면 중복 row를 구분할 수 없었다.

## 변경 사항

### 신규 세션 중복 방지

- `internal/service/session_builder.go`
  - 세션 조립 시 선택한 question ID를 추적한다.
  - review/new 문항 모두 동일한 append helper를 거쳐 중복을 제거한다.
  - repository가 중복을 반환하더라도 실제 추가된 문항 수만 잔여 slot에서 차감한다.
- `internal/repository/question_repo.go`
  - `GetNewQuestions`에 `excludeIDs`를 추가한다.
  - 카테고리별 relay와 fallback 조회 시 이미 고른 ID를 SQL에서 제외한다.

### 진행 중 세션 복구

- `internal/model/active_session.go`
  - `CurrentItemByQuestionID`를 추가해 Redis `CurrentIndex`의 문항만 답변 대상으로 인정한다.
- `internal/service/active_session.go`, `internal/service/grader.go`
  - 답변 기록과 grader lookup을 current-item 기준으로 변경한다.
- `internal/bot/session_answer.go`
  - Telegram callback 및 text answer lookup을 current-item 기준으로 변경한다.
- `internal/service/handwriting.go`, `internal/miniapp/handler.go`
  - 손글씨 submit 및 Telegram cleanup callback index 계산도 current-item 기준으로 변경한다.

callback format과 DB schema는 변경하지 않았다.

## 테스트

- `internal/service/session_builder_test.go`
  - review/new fetch에서 중복 ID가 반환돼도 세션에는 고유 ID만 저장되는지 검증한다.
- `internal/service/active_session_test.go`
  - 두 번째 duplicate occurrence가 현재 문항이면 해당 row만 답변 처리되는지 검증한다.
  - stale callback이 뒤쪽 duplicate occurrence를 대신 소비하지 않는지 검증한다.
- `internal/service/handwriting_test.go`
  - 손글씨 문항도 현재 duplicate occurrence를 정상 처리하는지 검증한다.

## 검증 결과

- `git diff --check`: PASS
- `go test ./internal/service ./internal/bot ./internal/miniapp`: PASS
- `make test`: PASS

## 로컬 런타임 복구

- tmux dashboard의 app pane만 `make dev`로 재기동했다.
- `GET /health`: PASS
- Redis working set `session:131:working_set`은 유지했다.
- 실패 과정에서 삭제된 text-answer 대기 키를
  `user:2006481393:active_question=131:13`으로 복원했다.
