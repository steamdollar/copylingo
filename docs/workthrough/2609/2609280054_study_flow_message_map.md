# StudyFlow·LLM 질문 메시지 locale map 통합

## 변경

- `internal/bot/bot_messages.go`의 단일 `botMessagesByLocale` 맵에 Study 안내·오류·완료 문구, 버튼·카드 레이블, LLM UI 문구와 Quiz/Study 컨텍스트 프롬프트를 모았다.
- `internal/bot/study_flow.go`, `internal/bot/llm_question.go`는 같은 맵을 사용한다. 사용자 언어 선택 기능은 추가하지 않았으며, 현재 출력은 기존 한국어 그대로다.

## 검증

- 한글 UI 문자열과 컨텍스트 프롬프트를 하나의 locale map에 모았다.
- `make test`: 통과.
- `make restart-app`: 성공, `http://localhost:8080/health` 준비 확인.
