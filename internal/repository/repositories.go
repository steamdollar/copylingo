package repository

import (
	"time"

	"github.com/jmoiron/sqlx"
)

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

func timeNow() time.Time {
	return time.Now()
}
