# 손글씨 client/server rebuild 정합성 검증 도구

## 배경

손글씨 Mini App은 사용자가 화면에서 그린 sampled stroke points를 서버에 JSON으로 제출하고, 서버는 같은 points를 PNG로 rebuild해 LLM에는 최종 PNG만 전달한다. client(Canvas 2D)와 server(Go custom rasterizer)는 서로 다른 renderer를 사용한다.

향후 false negative가 다시 발생할 경우 **LLM 판단 문제**와 **rebuild 손실 문제**를 분리하려면, 동일한 stroke JSON을 client Canvas와 서버 `RenderPNG()`에 입력해 획 연결·작은 mark·비율 유지 여부를 재현 가능하게 비교할 개발용 수단이 필요하다.

TODO 문서(`docs/todos/handwriting_rebuild_parity_verification.md`)에서 **Option A(개발용 export + renderer command)**로 확정된 범위를 구현했다.

## 변경 파일

- `cmd/dev/handwriting_renderer/main.go` (신규)
  - `cmd/dev/` 디렉터리 자체가 없어 함께 생성했다.
  - `-input`(stroke JSON) → `service.NewDefaultPNGStrokeRenderer().RenderPNG(strokes)` → `-output`(PNG) 로 변환하는 독립 CLI.
  - 입력 JSON은 Mini App debug export 포맷(`canvas_width`/`canvas_height`/`line_width`/`strokes`)을 그대로 받지만, renderer가 실제로 소비하는 건 `strokes` 뿐이다. 나머지 필드는 사람이 정합성 맥락을 읽기 위한 metadata로 carry만 한다.
  - error는 `fmt.Errorf("context: %w", err)` 패턴으로 wrap하고, boundary(`main`)에서 `log.Fatalf` 한 번만 찍는다(CONVENTIONS Go §3·§6).

- `web/miniapp/handwriting/app.js`
  - `?debug=1`일 때만 `setupDebugExport()`가 두 개의 debug 버튼(`client.png`, `strokes.json`)을 `document.body`에 추가한다. 운영 기본 UI에는 노출하지 않는다.
  - `client.png`: 현재 canvas를 `toBlob`으로 PNG export.
  - `strokes.json`: 서버 `RenderPNG()`에 들어가는 `state.strokes`를 그대로 직렬화하고, `canvas_width/height`와 `line_width = 10 * PAD_SCALE`(기존 stroke 두께)를 함께 담는다.
  - 기존 `lineWidth=10*PAD_SCALE`, `lineCap/lineJoin=round` drawing 로직은 건드리지 않았다.

- `internal/service/handwriting_render_test.go`
  - TODO "추가 후보" 2건만 보강했다(나머지 invariant는 이미 존재).
    - `TestPNGStrokeRendererKeepsBentStrokeConnected`: 한 stroke 안에서 급히 꺾이는 연속 경로(ㄱ 모양)가 꺾인 지점에서도 끊기지 않는다 → connected component 1개.
    - `TestPNGStrokeRendererDoesNotConnectSeparateStrokes`: 서로 떨어진 두 multi-point stroke를 서버가 임의로 잇지 않는다 → connected component 2개.
  - 기존 헬퍼(`renderTestPNG`, `blackPixelComponentCount`)를 재사용했다(DRY).

## 결정 사항

- Renderer dimensions 정책(`height=768`, `width=768~2304`), prompt, brush 산정 로직은 일절 건드리지 않았다(TODO "하지 말 것" 준수).
- 운영 환경 상시 PNG 저장(Option B)은 도입하지 않았다. export는 `?debug=1` 개발 경로에서만 동작한다.
- CLI 입력 포맷을 debug export 포맷과 일치시켜, Mini App에서 내보낸 `strokes.json`을 그대로 CLI에 넣을 수 있게 했다.

## 검증

- `go build ./...` 통과 (cmd/dev 포함).
- `go test ./internal/service` 통과 (추가 테스트 2건 포함, 개별 `-run`으로도 PASS 확인).
- `go vet ./cmd/dev/... ./internal/service/` 통과, `git diff --check` clean.
- CLI 실제 실행: 임의 샘플 stroke JSON(꺾인 stroke 1개 + 분리된 단일 점 1개)으로 `1018x768` PNG 생성 확인. 출력 PNG는 scratchpad에만 두고 커밋에서 제외했다.

## ⚠️ 남은 수동 검증 단계 (사용자 직접 — 자동화 불가)

최종 PNG **시각 비교**는 사람 눈이 필요해 이번 작업에 포함하지 않았다. 아래는 사용자가 직접 수행한다.

1. Mini App을 `?debug=1`로 열어 대표 stroke(꺾인 연속 stroke / 분리된 stroke / 탁점·반탁점 / `ヤ`·`や` / 다글자+작은 `ゃ` / 가로로 긴 단어)를 입력한다.
2. debug 버튼으로 `client.png`와 `strokes.json`을 내보낸다.
3. `go run ./cmd/dev/handwriting_renderer -input <strokes.json> -output server.png` 로 서버 rebuild PNG를 생성한다.
4. `client.png`와 `server.png`를 눈으로 대조한다. 비교 대상은 **획 연결·비율·작은 mark 보존**이다(서버는 bounding box 정규화를 하므로 여백·전체 크기는 달라도 정상).
