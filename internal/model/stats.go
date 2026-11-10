package model

type UserStats struct {
	UserID          int64   `json:"user_id"`
	TotalSessions   int     `json:"total_sessions"`
	TotalQuestions  int     `json:"total_questions"`
	TotalCorrect    int     `json:"total_correct"`
	OverallAccuracy float64 `json:"overall_accuracy"` // Percentage

	VocabularyAccuracy float64 `json:"vocabulary_accuracy"`
	GrammarAccuracy    float64 `json:"grammar_accuracy"`
	KanjiAccuracy      float64 `json:"kanji_accuracy"`
	ReadingAccuracy    float64 `json:"reading_accuracy"`
	ListeningAccuracy  float64 `json:"listening_accuracy"`

	CurrentStreak int `json:"current_streak"`

	TodayQuestions int `json:"today_questions"`
	TodayCorrect   int `json:"today_correct"`
}

type LevelProgress struct {
	Level            string `db:"level"`
	TotalMaterials   int    `db:"total_materials"`
	StudiedMaterials int    `db:"studied_materials"`
}

// Percent rounds down so a level reads 100% only when every material was studied.
func (progress LevelProgress) Percent() int {
	if progress.TotalMaterials == 0 {
		return 0
	}
	return progress.StudiedMaterials * 100 / progress.TotalMaterials
}
