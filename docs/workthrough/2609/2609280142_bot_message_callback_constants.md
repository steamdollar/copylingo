# Bot 문구·callback 상수 정리

## 변경

- `internal/bot/bot_messages.go`의 단일 한국어 locale map에 메뉴, 설정, Quiz, Study, 연결 자료 설정, 손글씨·어순 조립 문구를 모았다. 같은 버튼 문구는 기존 필드를 재사용한다.
- `internal/bot/bot_constants.go`에 bot 전용 명령어, callback 식별자·형식, LLM 허용 사용자 목록을 옮겼다. callback payload 문자열은 기존 값과 같다.
- 손글씨 callback 생성과 Mini App도 사용하는 `q:...:next`·`q:...:policy` 형식은 `internal/callback/callback.go`에 두었다. `internal/config/constants.go`에는 계층 간 공유하는 세션 상태와 Mini App 경로만 남겼다.
- `internal/bot`의 사용자 노출 한국어 문자열을 확인해 map 참조로 바꿨다. 로그, 주석, 테스트의 문자열과 함수 안에서만 쓰는 레이아웃 임계값은 대상에서 제외했다.

## 검증

- 수정한 Go 파일에 `goparams` 적용.
- `make test`: 통과.
- `make restart-app`: 성공, `http://localhost:8080/health` 준비 확인.
- `rg`로 `internal/bot`의 map 밖에 남은 한글이 주석뿐인 것을 확인했다.
