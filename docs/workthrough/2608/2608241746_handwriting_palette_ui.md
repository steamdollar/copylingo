# 손글씨 팔레트 UI 조정

## 변경 내용

- `web/miniapp/handwriting/app.js`: 셀 크기를 `224×280`에서 `190×238`로 약 85% 축소했다.
- `web/miniapp/handwriting/style.css`: 초기 canvas 크기와 배경 grid 크기를 같은 값으로 정렬했다.
- `web/miniapp/handwriting/index.html`: JS/CSS asset version을 갱신해 Telegram WebView의 stale cache를 우회한다.
- `web/miniapp/handwriting/app.js`: 채점 결과를 `✅`/`❌`로 간소화하고 정오와 무관하게 `정답: ...`을 표시한다.
- 정오표시와 정답은 팔레트 좌상단의 작은 overlay로 표시하고, 오답 feedback은 기존 status 영역에 유지한다.
- `internal/bot/session_question.go`: 손글씨 칸 수 계산에서 촉음 `っ`/`ッ`을 제외한다.
- `internal/bot/session_question_test.go`: 히라가나·가타카나 촉음과 일반 문자열의 칸 수 회귀 test를 추가했다.

## 판단

- 기존 가로:세로 비율과 2배 내부 해상도는 유지해 drawing 좌표·stroke 동작은 바꾸지 않았다.
- 정오 표시는 별도 문장 대신 emoji를 정답 텍스트 앞에 붙여 결과 영역 사용량을 줄였다.
- architecture나 policy 변경이 없는 국소 UI/계산 수정이므로 ADR은 추가하지 않았다.

## 검증

- `node --check web/miniapp/handwriting/app.js` — 통과
- `go test ./internal/bot -run 'TestHandwriting(MiniAppURL|CellCountExcludesSokuon)$'` — 통과
- `make test` — 통과
- `make restart-app` — 통과, `http://localhost:8080/health` ready 확인
- public tunnel의 HTML/asset hash 및 `?v=2608241815` 참조 확인 — 통과
