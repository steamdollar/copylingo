package model

// UserStats represents aggregated learning statistics.
type UserStats struct {
	UserID          int64   `json:"user_id"`
	TotalSessions   int     `json:"total_sessions"`
	TotalQuestions  int     `json:"total_questions"`
	TotalCorrect    int     `json:"total_correct"`
	OverallAccuracy float64 `json:"overall_accuracy"` // Percentage

	// Per-category accuracy
	VocabularyAccuracy float64 `json:"vocabulary_accuracy"`
	GrammarAccuracy    float64 `json:"grammar_accuracy"`
	KanjiAccuracy      float64 `json:"kanji_accuracy"`
	ReadingAccuracy    float64 `json:"reading_accuracy"`
	ListeningAccuracy  float64 `json:"listening_accuracy"`

	// Streak
	CurrentStreak int `json:"current_streak"`

	// Today
	TodayQuestions int `json:"today_questions"`
	TodayCorrect   int `json:"today_correct"`
}

// WeakArea represents a category/level combination the user struggles with.
type WeakArea struct {
	Category         QuestionCategory `json:"category"`
	ProficiencyLevel string           `json:"proficiency_level"`
	Accuracy         float64          `json:"accuracy"`
	Total            int              `json:"total"`
}
