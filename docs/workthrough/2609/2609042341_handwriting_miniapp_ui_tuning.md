# 손글씨 Mini App UI 추가 조정

## 변경 내용

- `web/miniapp/handwriting/app.js`: 손글씨 셀을 `170×224px`로 조정했다. 기존 기준 `224×280px` 대비 세로 약 80%이며, 가로는 기존 세로:가로 비율보다 약 5% 좁다.
- `web/miniapp/handwriting/style.css`: fallback canvas 크기와 grid 크기를 새 셀 크기에 맞췄다.
- `web/miniapp/handwriting/index.html`: 질문 영역을 `Q.` + 동적 prompt 한 줄 구조로 변경하고 정적 asset cache-buster를 갱신했다.

## 판단

- 실제 canvas 해상도는 기존과 같이 CSS 크기의 2배(`PAD_SCALE = 2`)를 유지해 stroke 좌표·서버 전송 동작은 변경하지 않았다.
- 질문 prompt는 기존 URL query에서 계속 받아 `stripPromptHTML`로 정제하며, 표시 label만 `문제`에서 `Q.`로 바꿨다.

## 검증

- `node --check web/miniapp/handwriting/app.js` — 통과
- `make test` — 통과
- `make restart-app` — 통과, `http://localhost:8080/health` ready 확인
- `curl http://localhost:8080/miniapp/handwriting` — 새 asset version 및 `Q.` markup 반영 확인
