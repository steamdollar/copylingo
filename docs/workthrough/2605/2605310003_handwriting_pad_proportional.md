# 손글씨 패드 폭을 글자 수에 비례시키기

## 배경

손글씨 채점 정확도가 낮다는 사용자 보고. 원인 분석 결과 두 축:

1. **입력 area 문제** — 캔버스가 720×320 고정이라 한 글자를 써도 글자가 패드 대비 작게 그려지고, 256px PNG로 정규화되며 탁점/반탁점·작은 가나 같은 미세 feature가 뭉개짐. 다글자 단어는 항상 가로 스크롤 필요.
2. **이미지 해상도/디테일 문제**(별건, 본 작업 범위 밖) — 렌더 256px + `Detail: low` 라 LLM이 ゛/゜, 작은 ャ 등을 오인. → 별도 제언으로 남김.

본 작업은 (1)의 입력 area를 **답안 글자 수에 비례**하도록 변경한다.

## 결정 사항

- 캔버스 폭 = `정사각형 셀(320px) × 글자 수`. 한 글자 = 정사각형 셀 1개.
- 글자 수는 **정답 문자열의 rune 수**(`len([]rune(CorrectAnswer))`)로 산정.
- **정답 문자열 자체는 client로 보내지 않는다** — cheat 방지. 길이(`cells`)만 query param으로 전달.
- `cells`는 server에서 최소 1로 clamp, client에서 1~8로 clamp.

## 변경 파일

- `internal/bot/session_question.go`
  - `handwritingMiniAppURL(...)` 시그니처에 `cells int` 추가, `cells` query param 세팅(최소 1 clamp).
  - 호출부에서 `cells := len([]rune(question.CorrectAnswer))` 계산해 전달.
- `internal/bot/session_question_test.go`
  - 시그니처 변경 반영 + `cells` param 검증 추가.
- `web/miniapp/handwriting/app.js`
  - `PAD_CELL_PX=320`, `PAD_HEIGHT_PX=320`, `PAD_MAX_CELLS=8` 상수.
  - `configurePad()` 추가: `cells` param을 읽어 `canvas.width/height`(버퍼) + `style.width/height` + `backgroundSize`(격자)를 글자 수에 맞춰 설정. canvas 리사이즈는 2D context 상태를 초기화하므로 stroke 속성(lineWidth 등)을 이 함수 안에서 다시 적용.
  - 기존 최상위 `ctx.lineWidth=...` 블록을 `configurePad()`로 이동.
- `web/miniapp/handwriting/index.html`
  - canvas 기본 attribute 720→320 (JS 적용 전 flash 방지), asset 버전 `v=2605310100`.
- `web/miniapp/handwriting/style.css`
  - `#pad` 기본값을 단일 정사각 셀(320, 격자 320)로. 실제 값은 `configurePad`가 인라인으로 덮어씀.

## 검증

- `go build ./...` OK
- `go test ./internal/bot/ ./internal/service/ ./internal/external/ ./internal/miniapp/` 전부 PASS.
- Mini App UI는 텔레그램 왕복이 필요해 수동 확인은 사용자 단에서 진행.

## 후속(별도 제언, 미적용)

- 채점 false negative의 근본 원인은 렌더 해상도(256px)와 `Detail: low`. → 렌더 size 상향(예: 512) + `Detail: high`, 프롬프트 leniency 강화를 사용자 승인 후 별도 작업으로 진행 예정.
