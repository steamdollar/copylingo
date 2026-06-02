# 손글씨 채점 정확도 튜닝 (Detail + Prompt + Renderer)

## 배경

손글씨 채점 false negative 완화를 위해 렌더 해상도를 512px로 높이고 LLM 이미지 입력을
`Detail: high`로 변경했으나, 기존 `internal/external/llm_test.go` assertion은 이전 정책을
계속 검증하고 있어 `make test`가 실패했다.

이후 실제 Mini App 사용에서 추가 false negative가 확인됐다.

- 맞게 쓴 답안이 획순 또는 작성 방향을 근거로 오답 처리됐다.
- `ン/ソ`, `シ/ツ`, `ヤ/や`, 탁점/반탁점처럼 static bitmap에서 애매한 입력을 엄격하게 판정했다.
- Mini App UI는 다글자 답안에 가로로 긴 canvas를 제공하지만, 서버는 모든 답안을 고정 `512x512` PNG로 rebuild해 작은 가나와 mark가 뭉개질 수 있었다.

이번 작업은 오늘 적용한 손글씨 이미지 품질 및 판정 정책 변경을 하나의 walkthrough로 통합한다.

## 변경 파일

- `internal/external/llm_test.go`
  - 이미지 detail 기대값을 `low`에서 `high`로 변경했다.
  - Conditional Verification prompt assertion을 현재 보수적 rejection 정책 문구와 맞췄다.
  - feedback assertion을 현재 correction note 제한 문구와 맞췄다.
  - Prompt provenance와 static PNG evidence boundary를 검증하는 회귀 테스트를 추가했다.
  - feedback Prompt와 strict JSON schema가 획순 관련 언급을 금지하는지 검증했다.
  - 요음 small kana tolerance rubric을 검증하는 회귀 테스트를 추가했다.
- `internal/external/llm.go`
  - Prompt에 static PNG 입력 provenance를 추가했다.
  - final visible bitmap만 평가하도록 제한했다.
  - 획순, 시작점, 작성 방향, pen movement 추론과 feedback 언급을 금지했다.
  - temporal pen-movement 정보가 있어야 구분할 수 있는 경우 `Expected Text`가 plausible하면 정답 처리하도록 명시했다.
  - script identity 또는 diacritic type이 애매한 경우에도 `Expected Text`가 plausible하면 정답 처리하도록 명시했다.
  - 요음의 작은 `ゃ/ゅ/ょ`, `ャ/ュ/ョ`는 textbook size, proportions, exact shape를 요구하지 않도록 명시했다.
  - expected 위치에 plausible한 두 번째 작은 mark가 있고 전체 `Expected Text`가 plausible하면 정답 처리하도록 명시했다.
  - 요음 오답 처리는 작은 kana가 명확히 없거나 unrelated shape로 명확히 대체된 경우로 제한했다.
  - strict JSON schema의 `feedback` description에도 같은 제한을 반영했다.
- `internal/service/handwriting_render.go`
  - 기본 Renderer 정책을 `height=512`, `width=512~1536`, `padding=48`로 정의했다.
  - `NewDefaultPNGStrokeRenderer()`를 추가해 기본 정책을 한 곳에서 관리한다.
  - 원본 stroke bounding box 비율에 따라 canvas 폭을 계산한다.
  - uniform scale로 전체 stroke를 fit해 stretch distortion을 방지한다.
  - 최대 폭 `1536px`로 Base64 upload 및 provider image processing 비용 증가를 제한한다.
- `internal/service/handwriting.go`
  - 기본 Renderer 생성 시 `NewDefaultPNGStrokeRenderer()`를 사용한다.
- `internal/service/handwriting_render_test.go`
  - 가로 입력에서 canvas 폭이 확장되는지 검증한다.
  - 폭이 최대값을 넘지 않는지 검증한다.
  - 출력 ink 비율이 원본 비율을 유지하는지 검증한다.
  - 작은 독립 mark가 rebuild 후 분리된 component로 남는지 검증한다.
- `docs/workthrough/2605/2605310003_handwriting_pad_proportional.md`
  - 512px 렌더와 `Detail: high`가 후속 적용된 상태를 반영했다.
- `docs/adr/ADR_from_01_to_20.md`
  - ADR-019와 ADR-020을 추가했다.
- `STATUS.md`
  - 완료 이력을 현재 문서 하나로 통합했다.
  - 남은 client/server rebuild 정합성 검증 TODO를 별도 문서로 연결했다.
- `docs/todos/handwriting_scoring_observability.md`
  - 앱 전체 Observability 설계 시점으로 보류하기로 결정해 기존 손글씨 전용 TODO를 삭제했다.

## 결정 사항

### Prompt Evidence Boundary

- Mini App은 sampled stroke points를 서버에 보내고, 서버는 이를 static PNG로 rebuild한다.
- LLM에는 최종 PNG만 전달되므로 획순, 시작점, 작성 방향, pen movement는 판정 근거로 사용할 수 없다.
- 혼동 문자 pair를 Prompt에 계속 추가하지 않고 범용 evidence boundary를 유지한다.
- 요음 small kana는 손가락 입력 특성을 고려해 textbook 형태와 비율을 강제하지 않는다.
- 요음은 expected 위치의 작은 mark가 plausible하면 정답 처리하고, 명확한 누락 또는 unrelated shape만 오답 처리한다.

### Bounded Aspect-Ratio Renderer

- 높이는 기존 검증값인 `512px`로 유지한다.
- 폭은 bounding box 비율에 따라 `512~1536px` 범위에서 확장한다.
- 비율 보존은 canvas dimensions와 uniform scale로 보장한다.
- `1536px` 상한은 calibration 시작점이다. 실제 품질·latency 확인 후 조정할 수 있다.

### 후속 작업

- 손글씨 전용 운영 로깅은 이번 범위에서 제외한다.
- 앱 전체 Observability 설계 시 함께 다룬다.
- client canvas와 서버 rebuild PNG의 정합성 검증은 별도 TODO로 남긴다.

## 검증 결과

```bash
go test ./internal/external
go test ./internal/service
make test
git diff --check
```

결과: 모두 통과.

실제 Mini App에서 변경 적용 후 정상 동작을 확인했다.
