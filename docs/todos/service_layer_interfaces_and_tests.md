# TODO: Service Layer 인터페이스 도입 + 단위 테스트 작성

## 배경 및 목적

현재 `internal/service/` 의 모든 service struct가 `*repository.XxxRepository` concrete 타입을 직접 필드로 들고 있어, DB 없이 service 레이어를 단위 테스트할 수 없다.

**목표**: 각 service 파일에 unexported 인터페이스를 추가하고, struct 필드/constructor 파라미터를 인터페이스로 교체한다. 이후 주요 service의 단위 테스트를 작성한다.

**원칙**:
- 인터페이스는 사용하는 쪽(service 패키지)에 정의 — `internal/repository/` 패키지는 손대지 않는다.
- concrete 타입이 인터페이스를 암묵적으로 만족하므로 **`services.go`의 콜 사이트는 변경 없다**.
- mock은 외부 라이브러리 없이 test 파일 내 수동 struct로 작성한다 (기존 `pipeline_test.go`의 `mockContentRepo` 패턴).
- 에러 처리는 `fmt.Errorf("context: %w", err)` 패턴 유지.

---

## Phase 1: 인터페이스 정의 및 struct 수정

아래 6개 파일을 수정한다. 각 파일 상단(import 블록 다음)에 인터페이스 블록을 추가하고, struct 필드와 constructor 파라미터 타입을 교체한다. import에서 `"github.com/lsj/copylingo/internal/repository"` 를 제거한다.

---

### 1. `internal/service/srs.go`

**추가할 인터페이스** (파일 상단에 추가):

```go
type questionQuerier interface {
    GetDueReviews(ctx context.Context, limit int) ([]model.Question, error)
    GetDueReviewCount(ctx context.Context) (int, error)
    UpdateSRS(ctx context.Context, q *model.Question) error
}

// srsScheduler는 GraderService와 SessionBuilderService가 SRSService에 의존할 때 쓰는 계약.
// *SRSService가 암묵적으로 만족한다.
type srsScheduler interface {
    GetDueReviews(ctx context.Context, limit int) ([]model.Question, error)
    GetDueCount(ctx context.Context) (int, error)
    ProcessAnswer(ctx context.Context, q *model.Question, isCorrect bool) error
}
```

**struct/constructor 변경**:

```go
// Before
type SRSService struct {
    questionRepo *repository.QuestionRepository
}
func NewSRSService(questionRepo *repository.QuestionRepository) *SRSService

// After
type SRSService struct {
    questionRepo questionQuerier
}
func NewSRSService(questionRepo questionQuerier) *SRSService
```

---

### 2. `internal/service/grader.go`

**추가할 인터페이스**:

```go
type graderUserRepo interface {
    UpdateStreak(ctx context.Context, userID int64) error
}

type graderQuestionRepo interface {
    GetByID(ctx context.Context, id int) (*model.Question, error)
    IncrementServed(ctx context.Context, id int) error
    IncrementCorrect(ctx context.Context, id int) error
}

type graderSessionRepo interface {
    Complete(ctx context.Context, id int, correctCount int) error
}

type graderSessionQuestionRepo interface {
    RecordAnswer(ctx context.Context, sessionID, questionID int, userAnswer string, isCorrect bool) error
    GetBySession(ctx context.Context, sessionID int) ([]model.SessionQuestion, error)
    GetWrongAnswers(ctx context.Context, sessionID int) ([]model.SessionQuestion, error)
}
```

**struct/constructor 변경**:

```go
// Before
type GraderService struct {
    userRepo            *repository.UserRepository
    questionRepo        *repository.QuestionRepository
    sessionRepo         *repository.SessionRepository
    sessionQuestionRepo *repository.SessionQuestionRepository
    srs                 *SRSService
    llm                 external.LLMClient
}
func NewGraderService(
    userRepo *repository.UserRepository,
    questionRepo *repository.QuestionRepository,
    sessionRepo *repository.SessionRepository,
    sessionQuestionRepo *repository.SessionQuestionRepository,
    srs *SRSService,
    llm external.LLMClient,
) *GraderService

// After
type GraderService struct {
    userRepo            graderUserRepo
    questionRepo        graderQuestionRepo
    sessionRepo         graderSessionRepo
    sessionQuestionRepo graderSessionQuestionRepo
    srs                 srsScheduler
    llm                 external.LLMClient
}
func NewGraderService(
    userRepo graderUserRepo,
    questionRepo graderQuestionRepo,
    sessionRepo graderSessionRepo,
    sessionQuestionRepo graderSessionQuestionRepo,
    srs srsScheduler,
    llm external.LLMClient,
) *GraderService
```

---

### 3. `internal/service/session_builder.go`

**추가할 인터페이스**:

```go
type questionFetcher interface {
    GetNewQuestions(ctx context.Context, language, level, category string, limit int) ([]model.Question, error)
    GetByID(ctx context.Context, id int) (*model.Question, error)
}

type sessionStore interface {
    CreateSession(ctx context.Context, s *model.Session) error
    GetByID(ctx context.Context, id int) (*model.Session, error)
    GetPendingSessions(ctx context.Context, userID int64) ([]model.Session, error)
    GetInProgressSessions(ctx context.Context, userID int64) ([]model.Session, error)
    Start(ctx context.Context, id int) error
}

type sessionQuestionStore interface {
    CreateSessionQuestions(ctx context.Context, sqs []model.SessionQuestion) error
    GetBySession(ctx context.Context, sessionID int) ([]model.SessionQuestion, error)
}
```

**struct/constructor 변경**:

```go
// Before
type SessionBuilderService struct {
    questionRepo        *repository.QuestionRepository
    sessionRepo         *repository.SessionRepository
    sessionQuestionRepo *repository.SessionQuestionRepository
    srs                 *SRSService
}
func NewSessionBuilderService(
    questionRepo *repository.QuestionRepository,
    sessionRepo *repository.SessionRepository,
    sessionQuestionRepo *repository.SessionQuestionRepository,
    srs *SRSService,
) *SessionBuilderService

// After
type SessionBuilderService struct {
    questionRepo        questionFetcher
    sessionRepo         sessionStore
    sessionQuestionRepo sessionQuestionStore
    srs                 srsScheduler
}
func NewSessionBuilderService(
    questionRepo questionFetcher,
    sessionRepo sessionStore,
    sessionQuestionRepo sessionQuestionStore,
    srs srsScheduler,
) *SessionBuilderService
```

---

### 4. `internal/service/analyzer.go`

**추가할 인터페이스**:

```go
type analyzerUserRepo interface {
    GetByID(ctx context.Context, id int64) (*model.User, error)
}

type sessionStatRepo interface {
    GetTodayStats(ctx context.Context) (int, int, error)
    GetCategoryAccuracy(ctx context.Context) (map[string]float64, error)
}
```

**struct/constructor 변경**:

```go
// Before
type AnalyzerService struct {
    userRepo            *repository.UserRepository
    sessionQuestionRepo *repository.SessionQuestionRepository
}
func NewAnalyzerService(userRepo *repository.UserRepository, sessionQuestionRepo *repository.SessionQuestionRepository) *AnalyzerService

// After
type AnalyzerService struct {
    userRepo            analyzerUserRepo
    sessionQuestionRepo sessionStatRepo
}
func NewAnalyzerService(userRepo analyzerUserRepo, sessionQuestionRepo sessionStatRepo) *AnalyzerService
```

---

### 5. `internal/service/user.go` (파일 이름 확인 필요)

UserService가 있는 파일. `GetUser`, `GetAllUsers` 등이 있을 것.

**추가할 인터페이스**:

```go
type userRepo interface {
    GetOrCreate(ctx context.Context, telegramID int64, username string) (*model.User, error)
    GetAllUsers(ctx context.Context) ([]model.User, error)
}
```

실제 UserService에서 사용하는 메서드만 포함한다. 파일을 확인하고 사용하지 않는 메서드는 제외한다.

---

### 6. `internal/service/handwriting.go`

**추가할 인터페이스**:

```go
type handwritingSessionRepo interface {
    GetByID(ctx context.Context, id int) (*model.Session, error)
}

type handwritingQuestionRepo interface {
    GetByID(ctx context.Context, id int) (*model.Question, error)
}

type handwritingSessionQuestionRepo interface {
    GetBySession(ctx context.Context, sessionID int) ([]model.SessionQuestion, error)
}

// graderClient는 HandwritingService가 GraderService에 의존할 때 쓰는 계약.
// *GraderService가 암묵적으로 만족한다.
type graderClient interface {
    GradeHandwriting(ctx context.Context, sessionID, questionID int, renderedImage []byte) (bool, string, error)
}
```

**struct/constructor 변경**: `grader *GraderService` → `grader graderClient`, 나머지 repo 필드도 인터페이스 타입으로 교체.

---

### `internal/service/services.go` — 변경 없음

`repos.Question`, `repos.Session` 등 concrete 타입이 인터페이스를 암묵적으로 만족하므로 콜 사이트는 그대로 둔다.

---

## Phase 2: 단위 테스트 작성

### Mock 작성 패턴

각 test 파일 내에 mock struct를 직접 선언한다. 예시:

```go
type mockGraderQuestionRepo struct {
    getByIDFn         func(ctx context.Context, id int) (*model.Question, error)
    incrementServedFn func(ctx context.Context, id int) error
    incrementCorrectFn func(ctx context.Context, id int) error
}

func (m *mockGraderQuestionRepo) GetByID(ctx context.Context, id int) (*model.Question, error) {
    return m.getByIDFn(ctx, id)
}
// ... 나머지 메서드 동일하게
```

---

### `internal/service/srs_test.go` (신규)

SM-2 알고리즘 검증. `ProcessAnswer`를 통해 `*model.Question`이 어떻게 변이되는지 확인.

mock `questionQuerier`는 `UpdateSRS`에서 인자로 받은 `*model.Question`을 캡처해 검증에 사용.

| 테스트 이름 | 초기 상태 | isCorrect | 검증 항목 |
|------------|----------|-----------|----------|
| `TestProcessAnswer_CorrectFirstRepetition` | Repetitions=0, IntervalDays=0 | true | IntervalDays=1, Repetitions=1 |
| `TestProcessAnswer_CorrectSecondRepetition` | Repetitions=1 | true | IntervalDays=6, Repetitions=2 |
| `TestProcessAnswer_CorrectSubsequentUsesFactor` | Repetitions=2, IntervalDays=6, EaseFactor=2.5 | true | IntervalDays=15 (6*2.5) |
| `TestProcessAnswer_WrongResetsRepetitions` | Repetitions=3, IntervalDays=20 | false | Repetitions=0, IntervalDays=1 |
| `TestProcessAnswer_EaseFactorFloorAt1_3` | EaseFactor=1.3 | false | EaseFactor >= 1.3 |
| `TestProcessAnswer_NextReviewAtSet` | 아무 상태 | true | NextReviewAt != nil |
| `TestProcessAnswer_UpdateSRSCalled` | - | true | mockQuestionQuerier.UpdateSRS 1회 호출 확인 |

---

### `internal/service/grader_test.go` (신규)

```go
// 필요한 mock
type mockGraderUserRepo struct { ... }
type mockGraderQuestionRepo struct { ... }
type mockGraderSessionRepo struct { ... }
type mockGraderSessionQuestionRepo struct { ... }
type mockSRS struct { ... }
type mockLLM struct { ... }  // external.LLMClient 인터페이스 구현
```

| 테스트 이름 | 시나리오 | 검증 항목 |
|------------|----------|----------|
| `TestGradeAnswer_Correct` | userAnswer == CorrectAnswer, 객관식 | RecordAnswer, IncrementServed, IncrementCorrect, ProcessAnswer 각 1회. 반환값 isCorrect=true |
| `TestGradeAnswer_Wrong` | userAnswer != CorrectAnswer, 객관식 | IncrementCorrect NOT 호출. ProcessAnswer(isCorrect=false) 1회 |
| `TestGradeAnswer_Subjective_Correct` | QuestionSubjective 타입, llm 반환 true | llm.GradeAnswer 1회 호출, isCorrect=true 전파 |
| `TestGradeAnswer_QuestionNotFound` | questionRepo.GetByID → 에러 | 에러 반환, 다운스트림 호출 없음 |
| `TestGradeAnswer_RecordAnswerFails` | RecordAnswer → 에러 | 에러 반환, IncrementServed/ProcessAnswer NOT 호출 |
| `TestCompleteSession_CorrectCountCalculation` | 3개 SessionQuestion (2개 IsCorrect=true, 1개 false) | sessionRepo.Complete(sessionID, 2) 호출 |
| `TestCompleteSession_StreakUpdated` | 정상 완료 | userRepo.UpdateStreak(userID) 1회 호출 |
| `TestCompleteSession_GetBySessionFails` | sessionQuestionRepo.GetBySession → 에러 | 에러 반환 |

---

### `internal/service/session_builder_test.go` (신규)

```go
type mockQuestionFetcher struct { ... }
type mockSessionStore struct { ... }
type mockSessionQuestionStore struct { ... }
type mockSRS struct { ... }
```

| 테스트 이름 | 시나리오 | 검증 항목 |
|------------|----------|----------|
| `TestBuildMorningSession_MixesReviewAndNew` | SRS → 4 reviews, questionRepo → 9 new | CreateSession 1회, CreateSessionQuestions 1회 (총 13개 entries) |
| `TestBuildMorningSession_NoDueReviews` | SRS → 0 reviews | GetNewQuestions 호출, 15개 new 시도 |
| `TestBuildSession_NoQuestionsReturnsNil` | SRS → 0, questionRepo → 0 | nil, nil 반환 |
| `TestBuildReviewSession_UsesOnlySRS` | BuildReviewSession(limit=5) | GetNewQuestions NOT 호출 |
| `TestBuildSession_SessionCreateFails` | sessionStore.CreateSession → 에러 | 에러 반환 |
| `TestBuildSession_SessionQuestionCreateFails` | CreateSessionQuestions → 에러 | 에러 반환 |

---

### `internal/service/analyzer_test.go` (신규)

| 테스트 이름 | 시나리오 | 검증 항목 |
|------------|----------|----------|
| `TestGetUserStats_MapsFieldsCorrectly` | user.StreakDays=5, todayStats=(10,7), categoryAcc={"vocabulary":80.0} | 모든 필드 매핑, OverallAccuracy=70.0 |
| `TestGetUserStats_ZeroDivision` | todayTotal=0 | OverallAccuracy=0 (panic 없음) |
| `TestGetWeakAreas_FiltersBelow60` | {vocabulary:59.9, grammar:60.0, kanji:75.0} | vocabulary만 반환 |
| `TestGetUserStats_UserNotFound` | userRepo.GetByID → 에러 | 에러 반환 |

---

## 검증 방법

1. `go build ./...` — 컴파일 에러 없음 확인
2. `make test` — 전체 테스트 통과 확인
3. concrete repo 타입들이 새 인터페이스를 만족하는지 컴파일 타임 확인:
   ```go
   // services.go 또는 별도 파일에 추가 (옵션):
   var _ srsScheduler = (*SRSService)(nil)
   var _ graderClient = (*GraderService)(nil)
   ```

---

## 주의사항

- `services.go`의 `NewServices` 함수 콜 사이트는 변경하지 않는다.
- `internal/repository/` 패키지는 건드리지 않는다.
- 인터페이스 이름은 lowercase unexported로 유지한다.
- mock struct의 function field가 nil인 채로 호출되면 panic이 나므로, 테스트마다 필요한 fn만 설정한다.
- `srsScheduler` 인터페이스는 `srs.go`에 정의하고 `grader.go`, `session_builder.go`에서 참조한다 (같은 package이므로 가능).
