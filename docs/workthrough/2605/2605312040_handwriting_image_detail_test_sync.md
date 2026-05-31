# 손글씨 이미지 디테일 테스트 정합성 복구

## 배경

손글씨 채점 false negative 완화를 위해 렌더 해상도를 512px로 높이고 LLM 이미지 입력을
`Detail: high`로 변경했으나, 기존 `internal/external/llm_test.go` assertion은 이전 정책을
계속 검증하고 있어 `make test`가 실패했다.

## 변경 사항

- `internal/external/llm_test.go`
  - 이미지 detail 기대값을 `low`에서 `high`로 변경했다.
  - Conditional Verification prompt assertion을 현재 보수적 rejection 정책 문구와 맞췄다.
  - feedback assertion을 현재 correction note 제한 문구와 맞췄다.
- `docs/workthrough/2605/2605310003_handwriting_pad_proportional.md`
  - 512px 렌더와 `Detail: high`가 후속 적용된 상태를 반영했다.

## 검증

```bash
go test ./internal/external
make test
git diff --check
```

결과: 모두 통과.
