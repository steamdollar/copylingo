package repository

import (
	"time"

	"github.com/jmoiron/sqlx"
)

// Repositories holds the DB adapters for their shared pool.
type Repositories struct {
	User               *UserRepository
	Content            *ContentRepository
	Material           *MaterialRepository
	MaterialPreference *MaterialPreferenceRepository
	Question           *QuestionRepository
	Session            *SessionRepository
	SessionQuestion    *SessionQuestionRepository
	QuizActiveSession  *QuizActiveSessionRepository
	StudyActiveSession *StudyActiveSessionRepository
	Tip                *TipRepository
}

// NewRepositories creates all repositories with the given DB connection.
func NewRepositories(db *sqlx.DB) *Repositories {
	return &Repositories{
		User:               NewUserRepository(db),
		Content:            NewContentRepository(db),
		Material:           NewMaterialRepository(db),
		MaterialPreference: NewMaterialPreferenceRepository(db),
		Question:           NewQuestionRepository(db),
		Session:            NewSessionRepository(db),
		SessionQuestion:    NewSessionQuestionRepository(db),
		QuizActiveSession:  NewQuizActiveSessionRepository(db),
		StudyActiveSession: NewStudyActiveSessionRepository(db),
		Tip:                NewTipRepository(db),
	}
}

// timeNow returns current time (extracted for testability).
func timeNow() time.Time {
	return time.Now()
}
