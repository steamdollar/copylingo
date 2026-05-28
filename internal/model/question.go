package model

import (
	"encoding/json"
	"time"
)

// QuestionType defines the type of question.
type QuestionType string

const (
	// QuestionMultipleChoice - 객관식 (4지선다 등)
	QuestionMultipleChoice QuestionType = "multiple_choice"
	// QuestionFillBlank - 빈칸 채우기
	QuestionFillBlank QuestionType = "fill_blank"
	// QuestionSubjective - 주관식 (자유 입력 및 AI 채점)
	QuestionSubjective QuestionType = "subjective"
	// QuestionKanaHandwriting - 가나 손글씨 입력 (Mini App + AI binary grading)
	QuestionKanaHandwriting QuestionType = "kana_handwriting"
	// QuestionTranslation - 번역 문제
	QuestionTranslation QuestionType = "translation"
	// QuestionListening - 청해 문제
	QuestionListening QuestionType = "listening"
	// QuestionReadingComp - 독해 문제
	QuestionReadingComp QuestionType = "reading_comp"
	// QuestionWordOrder - 단어 배열 (어순 교정)
	QuestionWordOrder QuestionType = "word_order"
)

// QuestionCategory defines the learning category.
type QuestionCategory string

const (
	CategoryKana        QuestionCategory = "kana"
	CategoryHandwriting QuestionCategory = "handwriting"
	CategoryVocabulary  QuestionCategory = "vocabulary"
	CategoryGrammar     QuestionCategory = "grammar"
	CategoryKanji       QuestionCategory = "kanji"
	CategoryReading     QuestionCategory = "reading"
	CategoryListening   QuestionCategory = "listening"
)

// Question represents a single learning question with embedded SRS state.
type Question struct {
	ID               int              `db:"id" json:"id"`
	ContentID        *int             `db:"content_id" json:"content_id"`
	Type             QuestionType     `db:"type" json:"type"`
	Language         string           `db:"language" json:"language"`                   // ISO 639-1: 'ja', 'el', 'en'
	ProficiencyLevel string           `db:"proficiency_level" json:"proficiency_level"` // JLPT: N5-N1, CEFR: A1-C2
	Category         QuestionCategory `db:"category" json:"category"`
	Prompt           string           `db:"prompt" json:"prompt"`
	Options          json.RawMessage  `db:"options" json:"options"`
	CorrectAnswer    string           `db:"correct_answer" json:"correct_answer"`
	Explanation      string           `db:"explanation" json:"explanation"`
	AudioPath        *string          `db:"audio_path" json:"audio_path"`
	Difficulty       int              `db:"difficulty" json:"difficulty"`
	TimesServed      int              `db:"times_served" json:"times_served"`
	TimesCorrect     int              `db:"times_correct" json:"times_correct"`
	// SRS (SM-2) state
	EaseFactor     float64    `db:"ease_factor" json:"ease_factor"`
	IntervalDays   int        `db:"interval_days" json:"interval_days"`
	Repetitions    int        `db:"repetitions" json:"repetitions"`
	NextReviewAt   *time.Time `db:"next_review_at" json:"next_review_at"`
	LastReviewedAt *time.Time `db:"last_reviewed_at" json:"last_reviewed_at"`
	CreatedAt      time.Time  `db:"created_at" json:"created_at"`
}

// GetOptions parses the JSONB options field into a string slice.
func (q *Question) GetOptions() ([]string, error) {
	var opts []string
	if err := json.Unmarshal(q.Options, &opts); err != nil {
		return nil, err
	}
	return opts, nil
}
