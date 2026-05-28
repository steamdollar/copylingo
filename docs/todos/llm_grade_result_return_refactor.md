# LLM 채점 반환값 구조체화

## 배경/목적

현재 `external.LLMClient` 의 채점 메서드는 다음처럼 tuple 반환을 사용한다.

```go
GradeAnswer(...) (bool, string, error)
GradeHandwriting(...) (bool, string, error)
```

의미:

- `bool` = 정답 여부
- `string` = feedback/advice
- `error` = 호출/파싱 실패

문제는 두 번째 `string`의 의미가 호출부에서 잘 드러나지 않고, 이미 `internal/external/llm.go`에 `GradeResult` struct 가 있는데도 함수 반환 시 다시 tuple 로 풀어서 반환한다는 점이다.

목표:

- LLM 채점 결과를 `(GradeResult, error)`로 반환하게 정리한다.
- feedback/advice 필드는 유지한다.
- 기존 public JSON field 이름 `feedback`은 유지한다.
- error sanitization 작업과 scope 를 섞지 않는다.

## 변경할 파일

- `internal/external/llm.go`
- `internal/service/grader.go`
- `internal/service/grader_test.go`
- 필요 시 관련 mock 이 있는 테스트 파일
- `docs/workthrough/YYMMDDhhmm_llm_grade_result_return_refactor.md`

건드리지 말 것:

- Mini App HTTP response JSON shape (`is_correct`, `feedback`, `correct_answer`, `explanation`)
- prompt 정책
- error taxonomy / error sanitization mapping
- DB schema / migration

## 현재 코드

### `internal/external/llm.go`

Before:

```go
type LLMClient interface {
	GradeAnswer(ctx context.Context, questionPrompt, correctAnswer, userAnswer string) (bool, string, error)
	GradeHandwriting(ctx context.Context, questionPrompt, correctAnswer string, pngImage []byte) (bool, string, error)
}
```

```go
var result GradeResult
if err := json.Unmarshal([]byte(rawContent), &result); err != nil {
	return false, "", fmt.Errorf("failed to parse llm handwriting output (%s): %w", rawContent, err)
}

return result.IsCorrect, result.Feedback, nil
```

### `internal/service/grader.go`

Before:

```go
isCorrect, feedback, err := g.llm.GradeHandwriting(ctx, question.Prompt, question.CorrectAnswer, renderedImage)
if err != nil {
	return false, "", err
}
```

## 결정된 구현 방향

### Option A — recommended

기존 `GradeResult`를 반환 타입으로 사용한다.

After:

```go
type LLMClient interface {
	GradeAnswer(ctx context.Context, questionPrompt, correctAnswer, userAnswer string) (GradeResult, error)
	GradeHandwriting(ctx context.Context, questionPrompt, correctAnswer string, pngImage []byte) (GradeResult, error)
}
```

```go
return result, nil
```

호출부:

```go
result, err := g.llm.GradeHandwriting(ctx, question.Prompt, question.CorrectAnswer, renderedImage)
if err != nil {
	return false, "", err
}

isCorrect := result.IsCorrect
feedback := result.Feedback
```

Pros:

- 반환값 의미가 명확하다.
- `Feedback` 필드를 유지하면서도 tuple 의미 혼동을 줄인다.
- 향후 `CorrectionNote`, `Confidence`, `RawModel`, `Latency` 같은 필드 확장이 쉽다.
- 이미 존재하는 `GradeResult`를 실제 public return type 으로 재사용한다.

Cons:

- interface, mock, service 호출부를 함께 수정해야 한다.
- `GradeAnswer`도 같이 바꾸면 영향 범위가 조금 넓어진다.

Metrics:

- 구현 공수: 45~60분
- 리스크: 낮음
- 타 코드 영향도: `external`, `service`, tests
- 유지보수 부담: 낮음

### Option B

손글씨만 먼저 `(GradeResult, error)`로 변경하고, 텍스트 `GradeAnswer`는 tuple 유지.

Pros:

- 현재 주력 경로인 손글씨부터 정리 가능.

Cons:

- `LLMClient` interface 내부에서 두 채점 메서드의 반환 스타일이 달라진다.
- 장기적으로 더 헷갈릴 수 있다.

Metrics:

- 구현 공수: 30분
- 리스크: 낮음
- 타 코드 영향도: 낮음
- 유지보수 부담: 중간

### Option C — Do nothing

현 상태 유지.

Pros:

- 구현 없음.

Cons:

- tuple 의미 혼동이 계속 남는다.
- `GradeResult` struct 를 내부 parse 용으로만 쓰는 어색한 구조가 유지된다.

Metrics:

- 구현 공수: 0
- 리스크: 낮음
- 유지보수 부담: 중간

Recommendation:

Option A. 두 LLM grading 경로의 반환 형태를 함께 정리하는 것이 가장 명확하다.

## 구현 상세

### 1. `external.LLMClient` interface 수정

```go
type LLMClient interface {
	GradeAnswer(ctx context.Context, questionPrompt, correctAnswer, userAnswer string) (GradeResult, error)
	GradeHandwriting(ctx context.Context, questionPrompt, correctAnswer string, pngImage []byte) (GradeResult, error)
}
```

### 2. `DefaultLLMClient` 메서드 반환 수정

Error path 예시:

```go
return GradeResult{}, config.ErrAIConfigMissing
```

Success path:

```go
return result, nil
```

### 3. `service.GraderService` 호출부 수정

`GradeAnswer` subjective branch:

```go
result, err := g.llm.GradeAnswer(ctx, question.Prompt, question.CorrectAnswer, userAnswer)
if err != nil {
	return false, "", err
}
isCorrect = result.IsCorrect
feedback = result.Feedback
```

`GradeHandwriting`:

```go
result, err := g.llm.GradeHandwriting(ctx, question.Prompt, question.CorrectAnswer, renderedImage)
if err != nil {
	return false, "", err
}
```

기존 `GraderService` public signature 는 유지한다.

```go
func (g *GraderService) GradeHandwriting(...) (bool, string, error)
```

이 TODO 는 LLM boundary 정리이며 service/API response shape 변경이 아니다.

### 4. 테스트 mock 수정

`internal/service/grader_test.go` 의 mock:

Before:

```go
gradeHandwritingFn func(ctx context.Context, prompt, correctAnswer string, image []byte) (bool, string, error)
```

After:

```go
gradeHandwritingFn func(ctx context.Context, prompt, correctAnswer string, image []byte) (external.GradeResult, error)
```

subjective test:

```go
return external.GradeResult{IsCorrect: true, Feedback: "Good job"}, nil
```

## 검증 방법

```bash
go test ./internal/external
go test ./internal/service
make test
```

## 완료 기준

- `external.LLMClient`의 두 grading 메서드가 `(GradeResult, error)`를 반환한다.
- `Feedback`/advice 의미는 유지된다.
- `GraderService` public method signature 와 Mini App response JSON 은 변경되지 않는다.
- 전체 테스트 통과.
