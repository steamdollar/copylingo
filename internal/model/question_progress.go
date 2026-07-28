package model

import "time"

const DefaultQuestionEaseFactor = 2.5

// UserQuestionProgress is the per-user SRS and aggregate state for one shared Question.
type UserQuestionProgress struct {
	UserID         int64      `db:"user_id"          json:"user_id"`
	QuestionID     int        `db:"question_id"      json:"question_id"`
	EaseFactor     float64    `db:"ease_factor"      json:"ease_factor"`
	IntervalDays   int        `db:"interval_days"    json:"interval_days"`
	Repetitions    int        `db:"repetitions"      json:"repetitions"`
	NextReviewAt   *time.Time `db:"next_review_at"   json:"next_review_at"`
	LastReviewedAt *time.Time `db:"last_reviewed_at" json:"last_reviewed_at"`
	TimesServed    int        `db:"times_served"     json:"times_served"`
	TimesCorrect   int        `db:"times_correct"    json:"times_correct"`
	CreatedAt      time.Time  `db:"created_at"       json:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at"       json:"updated_at"`
}

func NewUserQuestionProgress(userID int64, questionID int) UserQuestionProgress {
	return UserQuestionProgress{
		UserID:     userID,
		QuestionID: questionID,
		EaseFactor: DefaultQuestionEaseFactor,
	}
}
