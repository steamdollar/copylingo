# Telegram Word Order MVP

## 배경

- 선언만 있던 `QuestionWordOrder`를 실제 일본어 N5 Quiz pipeline에 연결했다.
- ADR-043에 따라 Mini App이 아닌 Telegram inline keyboard tap-to-build UX와 별도 Redis draft를 사용했다.

## 변경 내용

- `cmd/ja/catalog/data/n5_word_order.json`: 기존 grammar material에 연결된 static N5 Word Order 12문항을 추가했다.
- `cmd/ja/catalog/datasets.go`, `cmd/ja/seeder/main.go`, `cmd/ja/seeder/main_test.go`: embedded dataset, stable `question_key`, material link, chunk/correct-answer integrity 검증을 추가했다.
- `internal/bot/word_order.go`, `internal/bot/word_order_test.go`: deterministic shuffle, Redis draft, tap·undo·reset·submit, 반복 chunk index 처리와 focused tests를 추가했다.
- `internal/bot/session_question.go`, `internal/bot/session_flow.go`, `internal/config/constants.go`: type renderer, callback dispatch, session 종료 cleanup, callback/Redis key 규약을 연결했다.
- `docs/adr/ADR_from_41_to_60.md`, `STATUS.md`: ADR-043과 완료 상태를 기록했다.

## 결정과 trade-off

- 조립 중간 상태는 24시간 TTL Redis key에 option index 목록으로 보관하고 DB에는 최종 문장만 저장한다.
- session·question ID 기반 shuffle로 callback 사이의 표시 순서를 재현하고, 일본어 chunk를 공백 없이 join해 기존 exact grader·SRS를 재사용했다.
- Drag UI, 다국어 delimiter, 복수 정답, DB attempt telemetry는 MVP에서 제외했다.

## 검증

- `go test ./internal/bot ./cmd/ja/seeder ./cmd/ja/catalog ./internal/callback` — PASS
- `make test` — PASS
- `git diff --check` — PASS
- `make restart-app` — PASS (`http://localhost:8080/health` ready)
