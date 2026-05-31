# 손글씨 채점 Conditional Verification prompt 보강

## 배경

손글씨 Mini App 실제 사용 중 맞게 쓴 `ふ`, `オ`, `ニャ`, `びゃ`가 오답 처리되는 false negative 사례가 확인됐다.
특히 `オ`를 visually similar kanji인 `才`로 해석한 사례는 모델이 `Expected Text` 검증보다 대체 OCR transcription을 우선할 수 있음을 보여준다.

## 변경 파일

- `internal/external/llm.go`
  - 손글씨 채점 prompt를 `Expected Text` 기반 Conditional Verification으로 명시했다.
  - `Expected Text`가 plausible하면 대체 해석 가능성이 있어도 accept하도록 규칙을 추가했다.
  - 범용 규칙을 우선 기술하고 `オ`와 `才` 사례를 대표 예시 하나로 추가했다.
  - 정답 feedback은 empty string으로 유지하고, 오답 feedback은 Expected Text의 명확한 결손만 짧게 설명하도록 제한했다.
  - 대체 문자를 제안하거나 transcription하는 feedback은 금지했다.
- `internal/external/llm_test.go`
  - Conditional Verification 규칙과 대표 예시가 prompt에 포함되는지 검증했다.
  - feedback schema가 제한된 오답 correction note 정책을 반영하는지 검증했다.
  - provider가 반환한 오답 correction note가 호출자에게 전달되는지 검증했다.
- `docs/adr/ADR_from_01_to_20.md`
  - 기존 ADR-016을 덮어쓰지 않고 연관 ADR-018을 추가했다.
- `STATUS.md`
  - side task 완료 이력을 추가했다.

## 결정 사항

- 기존 ADR-016의 False Negative 최소화 방향을 유지한다.
- 여러 few-shot 예시를 누적하지 않는다.
- 특정 문자에 대한 anchoring을 줄이기 위해 범용 원칙을 먼저 제시하고 실제 실패 사례 하나만 보조 예시로 둔다.
- Mini App이 정답을 이미 표시하므로 대체 문자 설명은 생성하지 않는다.
- 오답 feedback은 Expected Text에서 명확히 누락되거나 잘못된 feature가 있을 때만 반환한다.

## 검증 결과

```bash
go test ./internal/external
make test
```

모두 통과.
