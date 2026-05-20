package repository

import (
	"time"

	"github.com/jmoiron/sqlx"
)

// Repositories holds all repository instances.
type Repositories struct {
	User            *UserRepository
	Content         *ContentRepository
	Question        *QuestionRepository
	Session         *SessionRepository
	SessionQuestion *SessionQuestionRepository
	Tip             *TipRepository
}

// NewRepositories creates all repositories with the given DB connection.
func NewRepositories(db *sqlx.DB) *Repositories {
	return &Repositories{
		User:            NewUserRepository(db),
		Content:         NewContentRepository(db),
		Question:        NewQuestionRepository(db),
		Session:         NewSessionRepository(db),
		SessionQuestion: NewSessionQuestionRepository(db),
		Tip:             NewTipRepository(db),
	}
}

// timeNow returns current time (extracted for testability).
func timeNow() time.Time {
	return time.Now()
}
