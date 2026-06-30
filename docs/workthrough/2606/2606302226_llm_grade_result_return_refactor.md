# LLM 채점 반환값 구조체화 — `(GradeResult, error)`

- **날짜**: 2026-06-30
- **Case**: B(구현/검증/종료) — 결정 방향은 TODO 문서에서 Option A로 확정
- **TODO**: `docs/todos/llm_grade_result_return_refactor.md`
- **요청**: `external.LLMClient`의 채점 메서드 2개 반환을 tuple `(bool, string, error)` → 기존 `external.GradeResult` struct `(GradeResult, error)`로 교체 (LLM boundary 정리)

## 배경 / 문제

- `LLMClient.GradeAnswer` / `GradeHandwriting`이 `(bool, string, error)` tuple 반환.
- 두 번째 `string`(=feedback/advice)의 의미가 호출부에서 드러나지 않음.
- 이미 `external.GradeResult{IsCorrect, Feedback}` struct가 존재하는데도 함수 반환 시 다시 tuple로 풀어서 내보내는 어색한 구조.
- 목표: 로직 변경 없이 시그니처/반환만 구조체화. `feedback` JSON field·prompt·error taxonomy·DB·Mini App response shape은 불변.

## 결정 (Option A, TODO 확정)

- `external.GradeResult`를 실제 public return type으로 재사용.
- `GraderService` public signature(`GradeAnswer/GradeHandwriting → (bool, string, error)`)와 Mini App JSON shape은 그대로 유지 — 이 작업은 LLM boundary 정리이지 service/API 변경이 아님.

## 변경 파일

| 파일 | 변경 |
|---|---|
| `internal/external/llm.go` | `LLMClient` interface 2개 메서드 + `DefaultLLMClient` 구현체 2개 반환을 `(GradeResult, error)`로 교체. 에러 path는 `GradeResult{}`, 성공 path는 `result` 반환 |
| `internal/service/grader.go` | `graderLLM` interface 2개 메서드 시그니처 교체 + 호출부 2곳(`GradeAnswerWithQuestion`, `GradeHandwritingWithQuestion`)에서 `result` 받아 `result.IsCorrect/result.Feedback` 분해 대입 |
| `internal/service/llm.go` | `LLMService.GradeAnswer/GradeHandwriting` wrapper 2개 반환 교체 (실제 호출처 없음·interface 정합용) |
| `internal/service/grader_test.go` | `mockLLM` 함수타입 필드 2개·메서드 2개 + 테스트 본문 3곳(`GradeResult{...}` 반환) |
| `internal/service/llm_test.go` | `mockLLMClient` 메서드 2개 + `external` import 추가 |
| `internal/bot/test_common_test.go` | `mockLLM` 함수타입·메서드 2개 + `external` import 추가 |
| `internal/bot/coverage_boost_test.go` | `gradeFn` 본문 1곳 + `external` import 추가 (TODO 사전분석 누락분) |
| `internal/external/llm_test.go` | 직접 호출 테스트 5곳(`GradeAnswer`/`GradeHandwriting` 결과를 `result.IsCorrect/Feedback`로, 결과 무시 케이스는 `_, err :=`로) (TODO 사전분석 누락분) |

## TODO 문서 대비 추가 수정 (컴파일/테스트 통과 필수)

사전분석에 명시된 6개 파일 외에 컴파일·테스트 통과를 위해 다음 2개 파일을 추가 수정:

- `internal/bot/coverage_boost_test.go` — `mockLLM.gradeFn`을 직접 작성하는 본문 1곳 + `external` import.
- `internal/external/llm_test.go` — `external` 패키지 내부 테스트로 `DefaultLLMClient` 채점 메서드를 직접 호출하는 테스트 5곳. `go build`는 `_test.go`를 컴파일하지 않아 빌드만으로는 안 잡혀, `go test` 단계에서 발견·수정.

## 범위 밖 (건드리지 않음, 의도적)

- `internal/service/handwriting_test.go`의 `mockGraderClient.GradeHandwritingWithQuestion` — GraderService 레이어 mock이라 반환 `(bool, string, error)` 유지가 정합. LLM client mock 아님.
- error sanitization / error taxonomy, prompt 정책, DB schema, Mini App HTTP response JSON shape.

## 검증

- `go build ./...` 통과.
- `go vet ./internal/external ./internal/service ./internal/bot` 클린.
- `go test ./internal/external ./internal/service ./internal/bot` 3개 패키지 전부 green.

## 주의 / 후속

- `LLMService`의 채점 wrapper 2개는 현재 실제 호출처가 없음(생성·DI 등록만). interface 정합을 위해 시그니처만 교체. 향후 사용 시 `external.GradeResult` 반환을 그대로 받게 됨.
- `GradeResult`에 `CorrectionNote`/`Confidence`/`Latency` 등 필드 확장 시 boundary 변경 없이 추가 가능(이번 작업의 직접 이득).
