# 손글씨 채점 결과 후 Tip Card 유지

## 배경/목적

손글씨 Mini App 은 제출 후 LLM 채점 대기 중 `loadingPanel` 안에서 spinner 와 tip card 를 보여준다. 현재는 응답이 오면 `finally { stopLoading(); }`가 실행되고, `stopLoading()`이 `loadingPanel.hidden = true`를 수행하여 표시 중이던 tip 도 함께 사라진다.

목표:

- 채점 응답이 온 뒤에도 마지막으로 표시된 tip card 는 계속 보이게 한다.
- spinner/“채점 중입니다” header 는 응답 후 사라지거나 비활성 상태로 바뀌어야 한다.
- tip rotation interval 은 응답 후 반드시 정지한다.
- tip fetch 실패/빈 tip 인 경우 현재처럼 tip 없이 동작한다.

## 현재 문제

### `web/miniapp/handwriting/index.html`

Before:

```html
<div id="loadingPanel" class="loading-panel" hidden>
  <div class="loading-header">
    <span class="spinner" aria-hidden="true"></span>
    <span class="loading-title">채점 중입니다</span>
  </div>
  <div id="tipCard" class="tip-card" aria-live="polite" hidden>
    <span class="tip-eyebrow" id="tipEyebrow"></span>
    <p class="tip-body" id="tipBody"></p>
  </div>
</div>
<p id="status" class="status">준비됨</p>
```

`tipCard`가 `loadingPanel` 안에 들어 있다. 따라서 `loadingPanel.hidden = true`가 되면 tip 도 사라진다.

### `web/miniapp/handwriting/app.js`

Before:

```js
function stopLoading() {
	tipState.loadingActive = false;
	if (tipState.intervalId) {
		clearInterval(tipState.intervalId);
		tipState.intervalId = null;
	}
	loadingPanel.hidden = true;
}
```

문제:

- interval 정지는 맞다.
- 하지만 panel 전체를 숨겨서 마지막 tip 을 보존하지 못한다.

## 변경할 파일

- `web/miniapp/handwriting/index.html`
- `web/miniapp/handwriting/app.js`
- `web/miniapp/handwriting/style.css`
- 필요 시 `docs/workthrough/YYMMDDhhmm_handwriting_tip_persist_after_result.md`

건드리지 말 것:

- backend tips API
- `internal/miniapp/handler.go`
- `internal/service/tip.go`
- DB schema / migration
- LLM 채점 로직

## 결정된 구현 방향

### Option A — recommended

`loadingHeader`와 `tipCard`를 분리 제어한다. `loadingPanel`은 tip container 역할을 유지하고, 응답 후에는 header 만 숨기며 tip card 는 남긴다.

Pros:

- DOM 이동이 최소다.
- 현재 tip fetch/cache/rotation 로직을 대부분 유지한다.
- 결과 message 와 tip card 를 같은 카드 안에서 자연스럽게 같이 보여줄 수 있다.

Cons:

- `loadingPanel`이라는 이름이 “로딩 전용”보다 넓은 의미가 된다. 추후 rename 여지가 있다.

Metrics:

- 구현 공수: 30~45분
- 리스크: 낮음
- 타 코드 영향도: 낮음
- 유지보수 부담: 낮음

### Option B

`tipCard`를 `loadingPanel` 밖으로 이동하고, loading panel 은 spinner 전용으로 유지한다.

Pros:

- DOM 의미가 명확하다.

Cons:

- CSS/layout 조정 범위가 조금 커진다.
- 기존 `loadingPanel.hidden`과 `tipCard.hidden` 상호작용을 더 많이 바꿔야 한다.

Metrics:

- 구현 공수: 45~60분
- 리스크: 낮음~중간
- 타 코드 영향도: frontend layout
- 유지보수 부담: 낮음

### Option C — Do nothing

현 상태 유지.

Pros:

- 변경 없음.

Cons:

- 사용자가 채점 결과를 읽는 순간 학습 tip 이 사라져, 대기 시간을 학습 맥락으로 전환하려던 UX 의 효과가 줄어든다.

Metrics:

- 구현 공수: 0
- 리스크: 낮음
- 타 코드 영향도: 없음
- 유지보수 부담: UX debt 유지

Recommendation:

Option A. 현재 구조를 가장 적게 흔들면서 원하는 UX 를 구현할 수 있다.

## 구현 상세

### 1. `index.html`에 loading header id 추가

After 예시:

```html
<div id="loadingPanel" class="loading-panel" hidden>
  <div id="loadingHeader" class="loading-header">
    <span class="spinner" aria-hidden="true"></span>
    <span class="loading-title">채점 중입니다</span>
  </div>
  <div id="tipCard" class="tip-card" aria-live="polite" hidden>
    <span class="tip-eyebrow" id="tipEyebrow"></span>
    <p class="tip-body" id="tipBody"></p>
  </div>
</div>
```

### 2. `app.js`에서 header 와 panel 을 분리 제어

After 예시:

```js
const loadingHeader = document.getElementById("loadingHeader");
```

```js
function startLoading() {
	tipState.loadingActive = true;
	loadingPanel.hidden = false;
	loadingHeader.hidden = false;
	startTipRotation();
}
```

```js
function stopLoading({ keepTip = false } = {}) {
	tipState.loadingActive = false;
	if (tipState.intervalId) {
		clearInterval(tipState.intervalId);
		tipState.intervalId = null;
	}
	loadingHeader.hidden = true;

	if (!keepTip || tipCard.hidden) {
		loadingPanel.hidden = true;
	}
}
```

`submitAnswer()`의 `finally`:

```js
} finally {
	stopLoading({ keepTip: true });
}
```

주의:

- 응답 성공/실패 모두 마지막 tip 을 유지해도 된다. 사용자가 provider error 를 본 경우에도 tip 은 부가 학습 컨텐츠로 남아 UX 를 해치지 않는다.
- tip 이 없으면 `tipCard.hidden === true`이므로 panel 은 숨겨진다.
- `clearPad()`는 tip 을 숨길지 유지할지 선택지가 있다. 기본 권장: 유지. 사용자가 결과/오류를 본 뒤 다시 쓰는 중에도 방금 본 tip 을 남겨도 UX 상 문제 없다.

### 3. CSS hidden rule 확인

현재:

```css
.loading-panel[hidden],
.tip-card[hidden] {
  display: none;
}
```

After 예시:

```css
.loading-panel[hidden],
.loading-header[hidden],
.tip-card[hidden] {
  display: none;
}
```

## 검증 방법

자동 테스트:

- 현재 `web/miniapp/handwriting`에는 JS unit test harness 가 없다. 이번 TODO 에서 새 test harness 를 도입하지 않는다.

수동 검증:

1. `make tmux` 또는 기존 dev server 로 Mini App 실행.
2. 손글씨 제출.
3. 채점 중 spinner 와 tip card 가 보이는지 확인.
4. 응답 도착 후:
   - spinner/“채점 중입니다”는 사라짐.
   - 결과 status 는 표시됨.
   - 마지막 tip card 는 그대로 표시됨.
   - tip 내용이 더 이상 rotation 되지 않음.
5. tip 데이터가 없는 환경:
   - 응답 후 빈 loading panel 이 남지 않아야 함.

검증 명령:

```bash
make test
```

프론트 변경이지만 Go 전체 회귀 확인 차원에서 실행한다.

## 완료 기준

- 채점 응답 후 마지막 tip card 가 유지된다.
- 응답 후 tip rotation interval 이 정지된다.
- tip 이 없을 때는 빈 panel 이 남지 않는다.
- `make test` 통과.
