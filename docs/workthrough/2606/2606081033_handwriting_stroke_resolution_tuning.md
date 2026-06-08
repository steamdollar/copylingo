# 손글씨 stroke 해상도 상향

## 배경

손글씨 kana 채점에서 맞게 쓴 답안이 오답 처리되는 false negative 사례가 계속 발생했다. 기존에는 Mini App canvas 내부 좌표가 셀당 `320px`, 서버 PNG renderer 기본 높이가 `512px`였다.

이번 변경은 LLM prompt/rubric은 유지하고, LLM에 전달되는 static PNG evidence의 품질을 높이는 좁은 튜닝이다.

## 변경 파일

- `web/miniapp/handwriting/app.js`
  - 화면상 pad 크기는 기존처럼 한 셀당 `320px`, 높이 `320px`로 유지한다.
  - 내부 canvas 좌표 해상도만 `PAD_SCALE=2`로 올려 셀당 `640px`, 높이 `640px`를 사용한다.
  - stroke line width도 scale에 맞춰 `10 * PAD_SCALE`로 조정해 상대 두께가 얇아지지 않게 했다.
- `web/miniapp/handwriting/index.html`
  - `app.js` cache-busting query를 `v=2606080100`으로 갱신했다.
- `internal/service/handwriting_render.go`
  - 서버 PNG renderer 기본 높이 `512 → 768`.
  - 최대 폭 `1536 → 2304`.
  - padding `48 → 72`.
  - 기존 height 비례 brush 산정 로직은 유지했다.
- `internal/service/handwriting_render_test.go`
  - `NewDefaultPNGStrokeRenderer()`가 새 기본 높이와 최대 폭 cap을 적용하는지 검증하는 회귀 테스트를 추가했다.

## 결정 사항

- Prompt policy는 건드리지 않았다. 이번 scope는 evidence 해상도 개선이다.
- Mini App의 표시 크기와 scroll UX는 유지했다. 사용자가 보는 입력 영역을 키우지 않고, 제출되는 stroke 좌표계만 고해상도로 바꿨다.
- 서버 renderer는 1.5배 상향(`512 → 768`)으로 제한했다. 비용/latency 증가를 고려해 `1024px`로 바로 올리지 않았다.

## 검증 결과

```bash
node --check web/miniapp/handwriting/app.js
go test ./internal/service
make test
```

결과: 모두 통과.

## 남은 리스크

- 서버 PNG가 커지므로 LLM 요청 payload와 image processing latency가 소폭 증가할 수 있다.
- 실제 false negative 개선 여부는 운영 입력으로 재확인이 필요하다.
- client/server rebuild parity 검증은 기존 TODO `docs/todos/handwriting_rebuild_parity_verification.md`에 남아 있다.
