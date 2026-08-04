# 손글씨 채점 acceptance-first prompt 보강

## 결론

손글씨 채점 prompt를 Expected Text 기준의 acceptance-first 정책으로 보강했다. 애매하거나 거친 PNG가 Expected Text와 plausibly 맞으면 정답으로 처리하고, 오답은 최종 bitmap에서 구체적 결함을 high confidence로 확인할 수 있을 때만 허용한다. `feedback`은 기본적으로 비워 불확실한 오답에서 추측성 교정을 노출하지 않는다.

## 배경 및 근거

- 기존 prompt에도 관대한 판정 지침은 있었지만, 2026-08-04 로그에서 `ゾ`, `ゆ`, `りゅ`, `シャワーをあびる` 등 시각적으로 plausibly 맞는 입력이 false verdict를 받은 사례가 확인됐다.
- Mini App은 false verdict의 `feedback`이 비어 있으면 정답 문자열만 표시한다. 따라서 불확실한 오답에서 빈 feedback을 유지하는 것이 추측성 설명보다 안전하다.
- 손글씨 요청에는 stroke timing이 없는 static PNG만 전달되므로 stroke order나 pen movement를 근거로 삼을 수 없다.

## 변경

- `internal/external/llm.go`
  - acceptance-first 기본값과 ambiguous/low-resolution/rough input 허용 규칙을 명시했다.
  - false는 하나의 구체적이고 관찰 가능한 결함을 high confidence로 설명할 수 있을 때만 허용한다.
  - feedback을 empty-by-default로 바꾸고, 불확실한 false verdict에는 빈 문자열을 반환하도록 했다.
  - response schema description도 동일한 정책으로 맞췄다.
- `internal/external/llm_test.go`
  - acceptance-first, concrete defect, empty-by-default feedback 계약을 문자열 테스트로 고정했다.
- `docs/adr/ADR_from_21_to_40.md`
  - ADR-039를 추가했다.

## 범위 외

- TTS 모델과 Mini App UI rendering은 변경하지 않았다. 현재 UI의 빈 feedback fallback이 “정답은 무엇인지”만 보여주는 요구를 충족한다.
- 사용자가 추가 작업 중이므로 서버/app 재시작과 live Gemini 호출은 실행하지 않았다.

## 검증

- `gofmt -w internal/external/llm.go internal/external/llm_test.go`
- `git diff --check`
- `go test ./internal/external`
- `make test`

모든 검증이 통과했다.
