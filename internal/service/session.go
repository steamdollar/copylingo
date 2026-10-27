package service

import (
	"context"

	"github.com/jmoiron/sqlx"

	"github.com/lsj/copylingo/internal/external"
	"github.com/lsj/copylingo/internal/model"
)

// SessionStores groups the typed Redis working-set stores for Quiz and Study.
// They stay separate instances (ADR-059 §8.6) and travel as one Deps field.
type SessionStores struct {
	Quiz  QuizSessionStore
	Study StudySessionStore
}

// QuestionRepo is the question-bank boundary used for Quiz selection and SRS counts.
type QuestionRepo interface {
	GetNewQuestions(
		ctx context.Context,
		userID int64,
		language string,
		levels []string,
		category string,
		excludeIDs []int,
		limit,
		kanjiRecallLimit int,
	) ([]model.Question, error)
	GetDueReviews(
		ctx context.Context,
		userID int64,
		language,
		currentLevel string,
		levels []string,
		limit,
		kanjiRecallLimit int,
		categories ...model.QuestionCategory,
	) ([]model.Question, error)
	GetDueReviewCount(
		ctx context.Context,
		userID int64,
		language string,
		levels []string,
	) (int, error)
}

// SessionRepo is the sessions-table boundary shared by both session modes.
type SessionRepo interface {
	CreateSession(
		ctx context.Context,
		s *model.Session,
	) error
	Start(
		ctx context.Context,
		id int,
	) error
	GetSessionsByStatus(
		ctx context.Context,
		userID int64,
		status model.SessionStatus,
	) ([]model.Session, error)
	ListInProgress(ctx context.Context) ([]model.Session, error)
	GetOldestUnfinished(
		ctx context.Context,
		userID int64,
	) (*model.Session, error)
	CountUnfinishedBatch(
		ctx context.Context,
		userIDs []int64,
	) (map[int64]int, error)
	CreateSessionInTx(
		ctx context.Context,
		tx *sqlx.Tx,
		session *model.Session,
	) (int, error)
	CreateSessionMaterialsInTx(
		ctx context.Context,
		tx *sqlx.Tx,
		sessionID int,
		materialIDs []int,
	) error
}

// SessionQuestionRepo stores the ordered question list of a new Quiz session.
type SessionQuestionRepo interface {
	CreateSessionQuestions(
		ctx context.Context,
		sqs []model.SessionQuestion,
	) error
}

// QuizActiveSessionRepo loads a Quiz working set from DB and flushes it back on completion.
type QuizActiveSessionRepo interface {
	LoadQuestionSessionWithStateBySessionID(
		ctx context.Context,
		sessionID int,
	) (*model.QuizActiveSessionState, error)
	FlushQuizActiveSession(
		ctx context.Context,
		state *model.QuizActiveSessionState,
	) error
}

// StudyActiveSessionRepo loads a Study working set from DB and flushes it back on completion.
type StudyActiveSessionRepo interface {
	LoadStudySessionWithStateBySessionID(
		ctx context.Context,
		sessionID int,
	) (*model.StudyActiveSessionState, error)
	FlushStudyActiveSession(
		ctx context.Context,
		state *model.StudyActiveSessionState,
	) error
}

// MaterialRepo selects Study materials for a session plan.
type MaterialRepo interface {
	GetMaterialsByPlan(
		ctx context.Context,
		userID int64,
		language,
		level string,
		levels []string,
		plan model.StudySessionPlan,
	) ([]model.Material, error)
}

// QuizGradingLLM grades handwriting answers, which need AI judgment.
type QuizGradingLLM interface {
	GradeHandwriting(
		ctx context.Context,
		questionPrompt,
		correctAnswer string,
		pngImage []byte,
	) (external.GradeResult, error)
}

// SessionUserRepo updates user-level learning records on Quiz completion.
type SessionUserRepo interface {
	UpdateStreak(
		ctx context.Context,
		userID int64,
	) error
}

// SessionDeps lists every external boundary SessionService touches (ADR-059 §8.3).
type SessionDeps struct {
	QuestionRepo           QuestionRepo
	SessionRepo            SessionRepo
	SessionQuestionRepo    SessionQuestionRepo
	QuizActiveSessionRepo  QuizActiveSessionRepo
	StudyActiveSessionRepo StudyActiveSessionRepo
	MaterialRepo           MaterialRepo
	UserRepo               SessionUserRepo
	Stores                 SessionStores
	LLM                    QuizGradingLLM
	// DB lets Study creation own its transaction boundary (ADR-061).
	DB *sqlx.DB
}

// SessionService is the Tier1 entry point for Quiz and Study sessions
// (ADR-059 §8.6). Callers see only this type; selection and progress are
// internal collaborators built here. Grading happens inline in the submit
// methods: exact match for text/option answers, the LLM for handwriting.
type SessionService struct {
	sessionRepo   SessionRepo
	userRepo      SessionUserRepo
	selection     *sessionBuilderService
	quizProgress  *quizActiveSessionService
	questionRepo  QuestionRepo
	llm           QuizGradingLLM
	studyBuilder  *studySessionService
	studyProgress *studyActiveSessionService
}

func NewSessionService(deps SessionDeps) *SessionService {
	quizProgress := newQuizActiveSessionService(
		deps.QuizActiveSessionRepo,
		deps.Stores.Quiz,
	)
	return &SessionService{
		sessionRepo: deps.SessionRepo,
		userRepo:    deps.UserRepo,
		selection: newSessionBuilderService(
			deps.QuestionRepo,
			deps.SessionRepo,
			deps.SessionQuestionRepo,
		),
		quizProgress: quizProgress,
		questionRepo: deps.QuestionRepo,
		llm:          deps.LLM,
		studyBuilder: newStudySessionService(
			deps.MaterialRepo,
			deps.SessionRepo,
			deps.DB,
		),
		studyProgress: newStudyActiveSessionService(
			deps.StudyActiveSessionRepo,
			deps.SessionRepo,
			deps.Stores.Study,
		),
	}
}
