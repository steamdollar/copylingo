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

// LevelProgress is how many of one level's materials a user has studied at
// least once in Study.
type LevelProgress struct {
	Level            string `db:"level"`
	TotalMaterials   int    `db:"total_materials"`
	StudiedMaterials int    `db:"studied_materials"`
}

// Percent rounds down so a level reads 100% only when every material was studied.
func (p LevelProgress) Percent() int {
	if p.TotalMaterials == 0 {
		return 0
	}
	return p.StudiedMaterials * 100 / p.TotalMaterials
}
