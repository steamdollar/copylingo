package model

import "time"

// SessionType defines the type of learning session.
type SessionType string

const (
	SessionMorning SessionType = "morning"
	SessionEvening SessionType = "evening"
	SessionReview  SessionType = "review"
	SessionArticle SessionType = "article"
)

// SessionStatus defines the current state of a session.
type SessionStatus string

const (
	SessionPending    SessionStatus = "pending"
	SessionInProgress SessionStatus = "in_progress"
	SessionCompleted  SessionStatus = "completed"
	SessionExpired    SessionStatus = "expired"
)

// Session represents a learning session containing a set of questions.
type Session struct {
	ID             int           `db:"id" json:"id"`
	UserID         int64         `db:"user_id" json:"user_id"`
	Type           SessionType   `db:"type" json:"type"`
	Status         SessionStatus `db:"status" json:"status"`
	TotalQuestions int           `db:"total_questions" json:"total_questions"`
	CorrectCount   int           `db:"correct_count" json:"correct_count"`
	StartedAt      *time.Time    `db:"started_at" json:"started_at"`
	CompletedAt    *time.Time    `db:"completed_at" json:"completed_at"`
	CreatedAt      time.Time     `db:"created_at" json:"created_at"`
}

// SessionQuestion represents a question entry within a session, including the user's answer.
type SessionQuestion struct {
	ID            int    `db:"id" json:"id"`
	SessionID     int    `db:"session_id" json:"session_id"`
	QuestionID    int    `db:"question_id" json:"question_id"`
	QuestionOrder int    `db:"question_order" json:"question_order"`
	IsReview      bool   `db:"is_review" json:"is_review"`
	UserAnswer    *string `db:"user_answer" json:"user_answer"` // NULL = not answered yet
	IsCorrect     *bool   `db:"is_correct" json:"is_correct"`   // NULL = not answered yet
}
