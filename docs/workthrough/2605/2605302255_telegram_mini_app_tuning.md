# Telegram Mini App tuning

## 배경

손글씨 Mini App 실사용 중 작은 UX 문제를 연속으로 확인했다.
별도 feature로 분리할 규모는 아니므로 frontend 중심의 tuning 작업으로 묶어 처리했다.

## Task 1. 채점 결과 후 Tip Card 유지

기존 `stopLoading()`은 응답 후 `loadingPanel` 전체를 숨겨 마지막 tip도 함께 사라지게 했다.

- loading header에 `loadingHeader` id를 추가하고 `loadingPanel`과 분리 제어한다.
- `stopLoading({ keepTip: true })` 호출 시 rotation interval과 loading header만 종료한다.
- tip이 실제로 표시된 경우 마지막 tip card를 유지한다.
- tip이 없으면 빈 loading panel을 숨긴다.
- 응답 후 `tipState.loadingActive = false`로 전환해 늦게 도착한 tip fetch가 rotation을 다시 시작하지 않게 했다.

## Task 2. Telegram vertical swipe 비활성화

canvas에서 아래 방향으로 획을 그을 때 Telegram Mini App이 최소화되는 문제가 있었다.

- Bot API 7.7 이상에서 `Telegram.WebApp.disableVerticalSwipes()`를 호출한다.
- Telegram header swipe를 이용한 최소화와 닫기는 계속 가능하다.

## Task 3. 단어 필기용 가로 canvas 확장

기존 `320x320` canvas는 단일 kana에는 충분하지만 짧은 kana 단어를 쓰기에는 좁았다.

- canvas 내부 좌표계를 `720x320`으로 확장했다.
- viewport는 모바일 화면 폭을 유지하고 내부 canvas만 가로로 넓혔다.
- canvas 아래 별도 `좌우 이동` slider를 추가했다.
- slider와 필기 gesture를 분리해 이동 중 오입력을 방지했다.
- backend renderer는 stroke bounding box를 비율 유지 정규화하므로 API와 DB는 변경하지 않았다.

## Task 4. Mini App 내부 문제 표시

Telegram 채팅에서 Mini App으로 진입하면 어떤 답안을 써야 하는지 다시 확인하기 어려웠다.

- Bot의 Mini App URL에 안내용 `prompt` query parameter를 추가했다.
- Mini App 상단에 문제 panel을 표시한다.
- prompt는 HTML tag를 제거한 뒤 `textContent`로 출력한다.
- URL prompt는 안내용이며 채점 SSOT가 아니다. 실제 채점은 기존처럼 서버 active session의 `question_id` 기준 원본 문항을 사용한다.

## 변경 파일

- `internal/bot/session_question.go`
- `internal/bot/session_question_test.go`
- `web/miniapp/handwriting/index.html`
- `web/miniapp/handwriting/app.js`
- `web/miniapp/handwriting/style.css`
- `docs/ARCHITECTURE.md`
- `STATUS.md`

CSS와 JavaScript asset version도 갱신해 browser cache를 무효화했다.

## 검증

```bash
node --check web/miniapp/handwriting/app.js
make test
git diff --check
```

결과: 모두 통과.

## 수동 확인 항목

- Mini App 진입 후 문제 prompt 표시
- 아래 방향 획 입력 시 Mini App이 최소화되지 않음
- slider로 canvas 좌우 이동 후 단어 필기 가능
- 채점 중 spinner와 tip card 표시
- 채점 완료 후 spinner 제거 및 마지막 tip 유지
- 채점 완료 후 tip rotation 정지
- tip이 없는 경우 빈 panel 미표시
