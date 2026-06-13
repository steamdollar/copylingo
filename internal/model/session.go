package model

import "time"

// SessionType defines the type of learning session.
type SessionType string

const (
	SessionMorning SessionType = "morning"
	SessionEvening SessionType = "evening"
	SessionReview  SessionType = "review"
	SessionArticle SessionType = "article"
	SessionStudy   SessionType = "study"
)

// SessionMode separates the interaction model from the session purpose.
type SessionMode string

const (
	SessionModeQuiz  SessionMode = "quiz"
	SessionModeStudy SessionMode = "study"
)

func (m SessionMode) IsValid() bool {
	return m == SessionModeQuiz || m == SessionModeStudy
}

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
	ID             int           `db:"id"              json:"id"`
	UserID         int64         `db:"user_id"         json:"user_id"`
	Type           SessionType   `db:"type"            json:"type"`
	Mode           SessionMode   `db:"mode"            json:"mode"`
	Status         SessionStatus `db:"status"          json:"status"`
	TotalQuestions int           `db:"total_questions" json:"total_questions"`
	CorrectCount   int           `db:"correct_count"   json:"correct_count"`
	StartedAt      *time.Time    `db:"started_at"      json:"started_at"`
	CompletedAt    *time.Time    `db:"completed_at"    json:"completed_at"`
	CreatedAt      time.Time     `db:"created_at"      json:"created_at"`
}

// SessionQuestion represents a question entry within a session, including the user's answer.
type SessionQuestion struct {
	ID            int     `db:"id"             json:"id"`
	SessionID     int     `db:"session_id"     json:"session_id"`
	QuestionID    int     `db:"question_id"    json:"question_id"`
	QuestionOrder int     `db:"question_order" json:"question_order"`
	IsReview      bool    `db:"is_review"      json:"is_review"`
	UserAnswer    *string `db:"user_answer"    json:"user_answer"` // NULL = not answered yet
	IsCorrect     *bool   `db:"is_correct"     json:"is_correct"`  // NULL = not answered yet
}

// SessionMaterial represents a material entry within a study session.
type SessionMaterial struct {
	ID            int        `db:"id"             json:"id"`
	SessionID     int        `db:"session_id"     json:"session_id"`
	MaterialID    int        `db:"material_id"    json:"material_id"`
	MaterialOrder int        `db:"material_order" json:"material_order"`
	StudiedAt     *time.Time `db:"studied_at"     json:"studied_at"`
	CreatedAt     time.Time  `db:"created_at"     json:"created_at"`
}

// StudySessionMaterial keeps the ordered session material and its material copy together.
type StudySessionMaterial struct {
	SessionMaterial SessionMaterial `json:"session_material"`
	Material        Material        `json:"material"`
}
