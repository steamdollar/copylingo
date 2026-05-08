# 에러 처리/로깅 리팩터링

## 작업 배경

손글씨 Mini App 세션 흐름을 리뷰하던 중 `showQuestion`에서 세션 문제 조회 실패와 정상 종료 조건이 하나의 분기로 묶여 있었다.

또한 Repository 계층에서 로그를 직접 찍을지, 에러 컨텍스트를 붙여 상위 계층에서 한 번만 로그를 찍을지에 대한 코드 작성 정책을 함께 정리했다.

## 변경 내용

### `showQuestion` 분기 정리

- `internal/bot/session_flow.go`의 `showQuestion`에서 `GetSessionQuestions` 에러와 `questionIdx >= len(sqs)` 정상 종료 조건을 분리했다.
- 세션 문제 조회 실패 시 서버 로그에 session ID와 에러를 남기고, 사용자에게는 "문제를 불러오지 못했습니다. 잠시 후 다시 시도해 주세요." 메시지를 보여주도록 했다.
- `questionIdx >= len(sqs)`는 기존처럼 "모든 문제를 풀었습니다" 완료 흐름으로 유지했다.

### Repository 에러 컨텍스트 정리

- `internal/repository/session_question_repo.go`의 DB 호출 에러에 `fmt.Errorf(...: %w)` 래핑을 추가했다.
- `CreateBatch`, `GetBySession`, `RecordAnswer`, `GetWrongAnswers`, `GetCategoryAccuracy`, `GetTodayStats`에서 실패 원인과 주요 식별자를 에러 메시지에 포함했다.
- Repository 계층에는 로그를 추가하지 않았다.
- 단순 에러 확인은 `if err := ...; err != nil` 또는 `if _, err := ...; err != nil` 형태로 정리했다.
- `GetTodayStats`는 named return의 `err`가 필요 없어져 `(int, int, error)` 반환 형태로 단순화했다.

### `session_questions` batch insert 정리

- `SessionQuestionRepository.CreateBatch`가 내부에서 단건 `Create`를 반복 호출하던 구조를 `sqlx.NamedExecContext` 기반 batch insert로 변경했다.
- `model.SessionQuestion`의 `db` tag를 사용해 `:session_id`, `:question_id`, `:question_order`, `:is_review` named parameter로 insert하도록 했다.
- batch 호출부에서 `session_questions.id`를 사용하지 않으므로 `RETURNING id`와 id scan 로직은 추가하지 않았다.
- 빈 slice 입력 시 바로 `nil`을 반환하는 no-op 동작을 유지했다.
- 수동 placeholder query builder가 필요 없어져 별도 helper와 테스트를 제거했다.
- 더 이상 사용되지 않는 단건 `Create` 함수는 제거했다.

### Service pass-through 기준 정리

- `internal/service/session_builder.go`의 `GetSessionQuestions`는 별도 비즈니스 의미를 추가하지 않는 단순 repository pass-through로 유지했다.
- Service 계층은 새로운 비즈니스 의미를 추가할 때만 에러를 래핑하고, 단순 위임 함수는 repository 에러를 그대로 반환하는 기준을 정했다.

### 정책 문서화

- `AGENTS.md`에 Go 에러 처리/로깅 정책을 추가했다.
- `CLAUDE.md`에도 동일한 에러 처리/로깅 정책을 반영했다.
- 정책 핵심은 다음과 같다.

```text
하위 계층은 로그를 찍지 않고 검색 가능한 에러 컨텍스트를 붙여 반환한다.
Bot handler, HTTP handler, scheduler job 같은 경계 계층에서 사용자/작업 맥락과 함께 한 번만 로그를 출력한다.
```

## 검증

- `gofmt -w internal/bot/session_flow.go`
- `gofmt -w internal/repository/session_question_repo.go internal/repository/question_repo_test.go internal/service/session_builder.go`
- `make test`

코드 변경 후 전체 테스트가 통과했다. Workthrough 통합과 문서 정리는 별도 테스트가 필요 없는 문서 변경으로 처리했다.
