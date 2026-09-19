# 긴 4지선다 선택지 세로 배치

## 변경

- `internal/bot/session_question.go`: 일반 객관식과 청해 문제가 함께 쓰는 키보드에서 선택지 하나라도 2열 폭 기준(ASCII 16칸, 비 ASCII 8글자 상당)을 넘으면 버튼을 한 행에 하나씩 배치한다. 선택지 순서와 답안 콜백 인덱스는 유지한다.
- `internal/bot/session_question_test.go`: 짧은 선택지, 길이 경계, 긴 일본어·영문 선택지, 홀수 선택지의 배치와 콜백을 검증한다.
- `STATUS.md`: 완료 항목을 기록한다.

## 판단

- 텔레그램 버튼 폭은 기기마다 다르므로 문자의 대략적인 화면 폭으로 분기한다. 기존 문제 풀이와 청해가 동일한 키보드 함수를 사용하므로 두 경로에 함께 적용된다.
- 별도 아키텍처 결정은 없으며 ADR 변경은 필요하지 않다.

## 검증

- `go test ./internal/bot -run 'TestBuildMCQKeyboardLayout|TestRenderByType' -count=1` 통과
- `make test` 통과
- `git diff --check` 통과
- `make restart-app` 완료, `http://localhost:8080/health` 준비 확인
