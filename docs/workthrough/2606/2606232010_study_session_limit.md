# Study Session 학습량 및 수동 개수 지정

## 배경

N1 목표 기준으로 현재 자동 Study Session 10개씩 2회는 신규 input 양이 부족할 수 있다.
자동 루틴은 살짝 증량하고, 사용자가 추가 학습을 원할 때 `/study` 명령어에서 직접 material 수를 지정할 수 있게 했다.

## 변경 파일

- `internal/service/study_session.go`
  - 기본 Study Session material 수를 10개에서 15개로 상향.
  - `BuildStudySessionWithLimit` 추가.
  - 수동 지정 limit 범위는 `1~50`으로 제한.
- `internal/bot/handler.go`
  - `/study [개수]` command argument 파싱 추가.
  - `/study`는 기본 15개, `/study 20`은 20개 Study Session 생성.
  - invalid argument는 사용법 메시지로 응답.
- `internal/service/study_session_test.go`
  - 기본 limit, 수동 limit, out-of-range limit 테스트 추가.
- `internal/bot/handler_dispatch_test.go`
  - `/study 20` 정상 생성과 `/study 999` 거부 테스트 추가.
  - command test helper가 Telegram command entity length를 command token에 맞추도록 보정.

## 결정 사항

- quiz 세션 크기는 이번 변경에서 유지했다.
  - 현재 quiz는 LLM/handwriting 채점 latency와 집중 피로도가 더 크므로, 먼저 Study 신규 input 양을 늘리고 수동 burst 기능으로 보완한다.
- 수동 limit 최대값은 50으로 제한했다.
  - Telegram UX와 단일 session 진행 부담을 고려한 guardrail이다.
- 신규 architecture 결정은 아니므로 ADR은 갱신하지 않았다.

## 검증

- `go test ./internal/service ./internal/bot`
- `git diff --check`
- `make test`

