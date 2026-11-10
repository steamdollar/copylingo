package service

import (
	"context"

	"github.com/jmoiron/sqlx"

	"github.com/lsj/copylingo/internal/external"
	"github.com/lsj/copylingo/internal/model"
)

// SessionStores groups the typed Redis working-set stores for Quiz and Study.
// They stay separate instances (ADR-059 §8.6) and travel as one SessionDependencies field.
type SessionStores struct {
	Quiz  QuizSessionStore
	Study StudySessionStore
}

// QuestionRepository is the question-bank boundary used for Quiz selection and SRS counts.
type QuestionRepository interface {
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

// SessionRepository is the sessions-table boundary shared by both session modes.
type SessionRepository interface {
	CreateSession(
		ctx context.Context,
		session *model.Session,
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

// SessionQuestionRepository stores the ordered question list of a new Quiz session.
type SessionQuestionRepository interface {
	CreateSessionQuestions(
		ctx context.Context,
		sessionQuestions []model.SessionQuestion,
	) error
}

// QuizActiveSessionRepository loads a Quiz working set from DB and flushes it back on completion.
type QuizActiveSessionRepository interface {
	LoadQuestionSessionWithStateBySessionID(
		ctx context.Context,
		sessionID int,
	) (*model.QuizActiveSessionState, error)
	FlushQuizActiveSession(
		ctx context.Context,
		state *model.QuizActiveSessionState,
	) error
}

// StudyActiveSessionRepository loads a Study working set from DB and flushes it back on completion.
type StudyActiveSessionRepository interface {
	LoadStudySessionWithStateBySessionID(
		ctx context.Context,
		sessionID int,
	) (*model.StudyActiveSessionState, error)
	FlushStudyActiveSession(
		ctx context.Context,
		state *model.StudyActiveSessionState,
	) error
}

// MaterialRepository selects Study materials for a session plan.
type MaterialRepository interface {
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

// SessionUserRepository updates user-level learning records on Quiz completion.
type SessionUserRepository interface {
	UpdateStreak(
		ctx context.Context,
		userID int64,
	) error
}

// SessionDependencies lists every external boundary SessionService touches (ADR-059 §8.3).
type SessionDependencies struct {
	QuestionRepository           QuestionRepository
	SessionRepository            SessionRepository
	SessionQuestionRepository    SessionQuestionRepository
	QuizActiveSessionRepository  QuizActiveSessionRepository
	StudyActiveSessionRepository StudyActiveSessionRepository
	MaterialRepository           MaterialRepository
	UserRepository               SessionUserRepository
	Stores                       SessionStores
	LLM                          QuizGradingLLM
	// DB lets Study creation own its transaction boundary (ADR-061).
	DB *sqlx.DB
}

// SessionService is the Tier1 entry point for Quiz and Study sessions
// (ADR-059 §8.6). Callers see only this type; selection and progress are
// internal collaborators built here. Grading happens inline in the submit
// methods: exact match for text/option answers, the LLM for handwriting.
type SessionService struct {
	sessionRepository  SessionRepository
	userRepository     SessionUserRepository
	selection          *sessionBuilderService
	quizProgress       *quizActiveSessionService
	questionRepository QuestionRepository
	llm                QuizGradingLLM
	studyBuilder       *studySessionService
	studyProgress      *studyActiveSessionService
}

func NewSessionService(dependencies SessionDependencies) *SessionService {
	quizProgress := newQuizActiveSessionService(
		dependencies.QuizActiveSessionRepository,
		dependencies.Stores.Quiz,
	)
	return &SessionService{
		sessionRepository: dependencies.SessionRepository,
		userRepository:    dependencies.UserRepository,
		selection: newSessionBuilderService(
			dependencies.QuestionRepository,
			dependencies.SessionRepository,
			dependencies.SessionQuestionRepository,
		),
		quizProgress:       quizProgress,
		questionRepository: dependencies.QuestionRepository,
		llm:                dependencies.LLM,
		studyBuilder: newStudySessionService(
			dependencies.MaterialRepository,
			dependencies.SessionRepository,
			dependencies.DB,
		),
		studyProgress: newStudyActiveSessionService(
			dependencies.StudyActiveSessionRepository,
			dependencies.SessionRepository,
			dependencies.Stores.Study,
		),
	}
}
