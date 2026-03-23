package model

import "time"

// User represents a Telegram user enrolled in the learning system.
type User struct {
	ID                 int64      `db:"id" json:"id"`                                     // Telegram user ID
	Username           string     `db:"username" json:"username"`                         // Telegram username
	Language           string     `db:"language" json:"language"`                         // ISO 639-1: 'ja', 'el', 'en'
	ProficiencyLevel   string     `db:"proficiency_level" json:"proficiency_level"`       // JLPT: N5-N1, CEFR: A1-C2
	StreakDays         int        `db:"streak_days" json:"streak_days"`                   // Consecutive study days
	StreakLastDate     *time.Time `db:"streak_last_date" json:"streak_last_date"`         // Last study date
	MorningSessionTime string     `db:"morning_session_time" json:"morning_session_time"` // Morning session time (HH:MM)
	EveningSessionTime string     `db:"evening_session_time" json:"evening_session_time"` // Evening session time (HH:MM)
	Timezone           string     `db:"timezone" json:"timezone"`                         // User timezone
}
