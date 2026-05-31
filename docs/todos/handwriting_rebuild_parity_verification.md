# TODO: 손글씨 Client/Server Rebuild 정합성 검증

## 배경 및 목적

손글씨 Mini App은 사용자가 화면에서 그린 sampled stroke points를 서버에 JSON으로 제출한다.
서버는 같은 points를 PNG로 rebuild하고, LLM에는 최종 PNG만 전달한다.

현재 client와 server는 서로 다른 renderer를 사용한다.

- Client: Canvas 2D API, `lineWidth=10`, `lineCap=round`, `lineJoin=round`
- Server: Go custom rasterizer, Bresenham-style line drawing, 원형 brush

Prompt evidence boundary와 bounded aspect-ratio Renderer를 적용한 뒤 실제 Mini App에서 정상 동작을 확인했다. 다만 향후 false negative가 다시 발생할 경우, LLM 판단 문제와 rebuild 손실 문제를 분리할 수 있도록 client canvas와 서버 PNG를 동일 입력 기준으로 비교하는 개발용 검증 수단이 필요하다.

**목표**: 동일한 stroke JSON을 client Canvas와 서버 `RenderPNG()`에 입력하고, 획 연결·작은 mark·비율이 유지되는지 재현 가능하게 비교한다.

---

## 변경할 파일

### 1. `web/miniapp/handwriting/app.js`

개발 환경에서만 사용할 parity export 수단을 추가한다.

- `?debug=1`일 때만 활성화한다.
- 현재 canvas의 PNG와 stroke JSON을 내보낸다.
- 운영 기본 UI에는 노출하지 않는다.

JSON 예시:

```json
{
  "canvas_width": 320,
  "canvas_height": 320,
  "line_width": 10,
  "strokes": []
}
```

### 2. `cmd/dev/handwriting_renderer/main.go`

개발용 command를 추가한다.

```bash
go run ./cmd/dev/handwriting_renderer \
  -input tmp/handwriting-parity/handakuten/strokes.json \
  -output tmp/handwriting-parity/handakuten/server.png
```

- 입력 JSON에서 `strokes`를 읽는다.
- `service.NewDefaultPNGStrokeRenderer()`로 PNG를 생성한다.
- 출력 파일 경로에 서버 rebuild PNG를 저장한다.

### 3. `internal/service/handwriting_render_test.go`

필요한 invariant 테스트가 부족하면 보강한다.

현재 이미 검증하는 항목:

- 가로 입력에서 canvas 폭 확장
- 최대 폭 제한
- 출력 ink 비율 유지
- 작은 독립 mark 보존

추가 후보:

- 꺾인 연속 stroke가 끊기지 않음
- 서로 분리된 stroke가 불필요하게 연결되지 않음

---

## 비교 절차

개발 환경에서 동일 stroke JSON을 기준으로 다음을 비교한다.

```text
tmp/handwriting-parity/<case>/
  strokes.json
  client.png
  server.png
```

확인할 fixture:

| 입력 | 확인할 내용 |
|---|---|
| 꺾인 연속 stroke | 서버 PNG에서 선이 끊기지 않음 |
| 서로 분리된 stroke | 서버가 획 사이를 임의로 연결하지 않음 |
| 탁점·반탁점 | 작은 독립 mark가 사라지거나 합쳐지지 않음 |
| `ヤ/や` | script 구분에 필요한 visible shape가 유지됨 |
| 다글자 + 작은 `ゃ` | 글자 비율과 작은 가나 크기가 유지됨 |
| 가로로 긴 단어 | bounded width 안에서 원본 비율이 최대한 유지됨 |

서버는 bounding box 정규화를 수행하므로 여백과 전체 크기는 달라도 된다. 비교 대상은 획 연결, 비율, 작은 mark 보존이다.

---

## Options

| Option | 내용 | Pros | Cons |
|---|---|---|---|
| **A (Recommended)** | 개발용 export + renderer command | 운영 영향 없이 실제 입력 재현 가능 | 개발용 command와 debug UI 유지 필요 |
| B | 운영 환경에서 rendered PNG 상시 저장 | 실제 실패 사례 분석이 쉬움 | 개인정보·저장 주기·파일 관리 정책 필요 |
| C | Do nothing | 변경 없음 | 향후 Renderer 문제를 추측에 의존 |

---

## 하지 말 것

- 운영 환경에서 PNG를 상시 저장하지 않는다.
- 앱 전체 Observability 시스템을 이번 TODO에 포함하지 않는다.
- Renderer dimensions 정책(`height=512`, `width=512~1536`)을 변경하지 않는다.
- Prompt를 추가 수정하지 않는다.

---

## 검증 방법

```bash
go test ./internal/service
make test
git diff --check
```

수동 검증:

1. Mini App을 `?debug=1`로 열어 대표 stroke를 입력한다.
2. `strokes.json`과 `client.png`를 내보낸다.
3. 개발용 command로 `server.png`를 생성한다.
4. client/server PNG의 획 연결, 작은 mark, 비율을 비교한다.
