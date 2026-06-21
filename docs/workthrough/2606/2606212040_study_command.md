# `/study` Study Session 즉시 생성 command 추가

## 배경

Telegram에서 `/study` 명령어를 실행하면 Study Material 기반 세션 하나를 즉시 생성하고 시작 버튼을 발송하는 기능이 필요했다. 기존 scheduler/admin 흐름은 이미 `StudySessionService.BuildStudySession`을 사용하고 있었으므로, 새 생성 로직을 만들지 않고 동일 service를 command entrypoint에서 재사용했다.

## 변경 파일

- `internal/config/constants.go`
  - `CommandStudy` 추가.
- `internal/bot/handler.go`
  - `/study` dispatch 추가.
  - `handleStudy` 추가: 사용자 조회, Study Session 생성, 기존 `PushStudySession` 재사용.
  - `/help` 명령어 목록에 `/study` 추가.
- `internal/bot/handler_dispatch_test.go`
  - `/study` 성공, material 없음, user lookup 실패, build 실패, push 실패 테스트 추가.
- `internal/bot/test_common_test.go`
  - Telegram send 실패 경로 테스트를 위해 `mockBotAPI.sendErr` 추가.

## 결정 사항

- `/study`는 새 Study Session을 생성한 뒤 기존 StudyFlow push 메시지를 보낸다.
- pending/in_progress material 제외 정책은 `MaterialRepository.GetForStudySession`의 기존 SQL을 따른다.
- Study Session material count는 기존 `StudySessionService`의 8개 정책을 그대로 따른다.
- ADR 갱신 없음. 신규 architecture 결정이 아니라 기존 Study Session 생성 경로 재사용이다.

## 검증

- `go test ./internal/bot`
- `go test ./internal/bot ./internal/service`
- `make test`
- `make restart-app`
  - `http://localhost:8080/health` ready 확인 완료.
