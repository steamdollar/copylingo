package model

import "time"

// SessionSlot represents one of the four personalized daily schedule slots.
type SessionSlot string

const (
	SessionSlotMorningStudy SessionSlot = "morning_study"
	SessionSlotMorningQuiz  SessionSlot = "morning_quiz"
	SessionSlotEveningStudy SessionSlot = "evening_study"
	SessionSlotEveningQuiz  SessionSlot = "evening_quiz"
)

// User represents a Telegram user enrolled in the learning system.
type User struct {
	ID                 int64      `db:"id"                   json:"id"`                   // Telegram user ID
	Username           string     `db:"username"             json:"username"`             // Telegram username
	Language           string     `db:"language"             json:"language"`             // ISO 639-1: 'ja', 'el', 'en'
	ProficiencyLevel   string     `db:"proficiency_level"    json:"proficiency_level"`    // JLPT: N5-N1, CEFR: A1-C2
	StreakDays         int        `db:"streak_days"          json:"streak_days"`          // Consecutive study days
	StreakLastDate     *time.Time `db:"streak_last_date"     json:"streak_last_date"`     // Last study date
	MorningSessionTime string     `db:"morning_session_time" json:"morning_session_time"` // Legacy Morning session time (HH:MM)
	EveningSessionTime string     `db:"evening_session_time" json:"evening_session_time"` // Legacy Evening session time (HH:MM)
	MorningStudyTime   *string    `db:"morning_study_time"   json:"morning_study_time"`   // Morning study time (HH:MM), nil if disabled
	MorningQuizTime    *string    `db:"morning_quiz_time"    json:"morning_quiz_time"`    // Morning quiz time (HH:MM), nil if disabled
	EveningStudyTime   *string    `db:"evening_study_time"   json:"evening_study_time"`   // Evening study time (HH:MM), nil if disabled
	EveningQuizTime    *string    `db:"evening_quiz_time"    json:"evening_quiz_time"`    // Evening quiz time (HH:MM), nil if disabled
	Timezone           string     `db:"timezone"             json:"timezone"`             // User timezone
}
