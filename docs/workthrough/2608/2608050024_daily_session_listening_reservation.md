# Daily Quiz Session Listening 예약

## 배경

Listening Question 50개와 audio가 준비돼 있었지만, 신규 Listening은 Random Slot Relay의 다섯 번째 category라 실제 노출이 매우 적었다. Evening은 기존 10자리를 review와 Vocabulary가 모두 사용해 relay 자체가 실행되지 않는 경우가 일반적이었다.

## 결정

- Morning Session을 15개에서 17개로 늘린다.
- Evening Session을 10개에서 12개로 늘린다.
- 각 Daily Session에서 audio-ready unseen Listening을 최대 1개 먼저 예약한다.
- Listening 후보가 없으면 해당 자리는 기존 Random Slot Relay가 채운다.
- Vocabulary 최소 1/3, Kanji recall 최대 3, Reading 최대 1 정책은 유지한다.

## 변경 파일

- `internal/service/session_builder.go`: 총량과 Listening 예약 단계를 추가했다.
- `internal/service/session_builder_test.go`: Morning 17문항, Evening 12문항, Listening 1자리 예약과 fallback을 검증했다.
- `docs/adr/ADR_from_41_to_60.md`: ADR-041을 기록했다.
- `AGENTS.md`: 최신 ADR range를 41~60으로 갱신했다.
- `STATUS.md`: 최근 완료 항목을 추가했다.

## 검증

- `go test ./internal/service -run 'TestBuild(Morning|Evening)Session' -count=1 -v`: 통과
- `go test ./internal/service -count=1`: 통과
- `make test`: 전체 통과
- `make restart-app`: 성공 (`[copylingo] app is ready`)
- `curl -fsS http://localhost:8080/health`: `{"status":"healthy", ...}` 확인
