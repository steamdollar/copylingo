# TODO: 손글씨 채점 관측 로깅 보강

## 배경 및 목적

`kana_handwriting` 채점 latency와 cost 최적화 논의 중 다음 사실을 확인했다.

- 현재 실전 로그에는 `render`, `grade`, `llm`, `record`, `submit total`, `image_bytes` 정도만 남는다.
- 실제 확인된 샘플에서는 `render`가 20~30ms 수준이고 `llm`이 2~7초 수준이라, 전체 latency 대부분은 LLM multimodal 호출 구간으로 보인다.
- 하지만 `strokes`/`points` 입력량과 latency 또는 `image_bytes`의 상관관계를 판단할 데이터가 없다.

**목표**: point limit, stroke simplification, renderer 조정 같은 최적화 전에 근거가 되는 최소 관측 데이터를 남긴다.
검증 필요 가설: stroke의 points수를 통제해서 줄이면 server side handling의 cost가 줄어드는가?

---

## 변경할 파일

### 1. `internal/service/handwriting.go`

현재 로그:

```go
log.Printf("[Handwriting] service total=%s render=%s grade=%s session_id=%d question_id=%d image_bytes=%d",
    time.Since(startedAt), renderedAt.Sub(startedAt), time.Since(renderedAt), req.SessionID, req.QuestionID, len(renderedImage))
```

변경 후 예시:

```go
strokeCount, pointCount := handwritingStrokeStats(req.Strokes)

log.Printf("[Handwriting] service total=%s render=%s grade=%s session_id=%d question_id=%d strokes=%d points=%d image_bytes=%d",
    time.Since(startedAt), renderedAt.Sub(startedAt), time.Since(renderedAt), req.SessionID, req.QuestionID, strokeCount, pointCount, len(renderedImage))
```

`handwritingStrokeStats` helper는 같은 파일 또는 `internal/service/handwriting_render.go`에 둔다.

```go
func handwritingStrokeStats(strokes []Stroke) (strokeCount int, pointCount int) {
    strokeCount = len(strokes)
    for _, stroke := range strokes {
        pointCount += len(stroke.Points)
    }
    return strokeCount, pointCount
}
```

### 2. `internal/service/handwriting_test.go` 또는 `internal/service/handwriting_render_test.go`

helper 단위 테스트 추가:

```go
func TestHandwritingStrokeStats(t *testing.T) {
    strokes := []Stroke{
        {Points: []StrokePoint{{X: 1, Y: 1}, {X: 2, Y: 2}}},
        {Points: []StrokePoint{{X: 3, Y: 3}}},
    }

    strokeCount, pointCount := handwritingStrokeStats(strokes)

    if strokeCount != 2 || pointCount != 3 {
        t.Fatalf("stats = (%d, %d), want (2, 3)", strokeCount, pointCount)
    }
}
```

---

## 수집할 데이터

실제 Mini App 사용 10~20건 정도에서 다음 값을 비교한다.

| 값 | 목적 |
|---|---|
| `strokes` | 획 수가 많을수록 renderer/LLM 입력이 복잡해지는지 확인 |
| `points` | 브라우저 pointer sampling 편차와 payload 규모 확인 |
| `image_bytes` | PNG 크기가 LLM latency와 상관 있는지 확인 |
| `render` | 서버 CPU 비용 확인 |
| `llm` / `grade` | provider-side 병목 확인 |
| `is_correct` | false negative/false positive 체감 사례와 연결 |

---

## 하지 말 것

- point count cap, distance filter, stroke simplification은 이번 TODO 범위가 아니다.
- renderer size(`256x256`) 변경 금지.
- LLM model / prompt / `MaxCompletionTokens` 추가 변경 금지.
- 로그 수집 전 “point 수가 많아서 느리다”는 결론을 내리지 말 것.

---

## 의사결정 / 결정된 사항

- 현 시점에서 input point limit은 latency 최적화 근거가 부족하다.
- server-side cap은 보안/예측 가능성 측면에서는 정당화 가능하지만, 별도 결정이 필요하다.
- 우선 관측 로깅만 추가하고, 데이터가 쌓인 뒤 다음 중 하나를 결정한다.
  - Do nothing
  - client-side minimum distance filter
  - server-side point cap
  - renderer/stroke simplification
  - 모델 교체 A/B

---

## 검증 방법

```bash
go test ./internal/service
make test
```

수동 검증:

1. `make dev` 또는 tmux app pane 재시작.
2. Mini App에서 손글씨 문항 3건 이상 제출.
3. app 로그에 다음 형태가 찍히는지 확인:

```text
[Handwriting] service total=... render=... grade=... session_id=... question_id=... strokes=... points=... image_bytes=...
```

4. 기존 `llm elapsed`, `grader total`, `submit total` 로그와 시간 흐름이 일관적인지 확인.
