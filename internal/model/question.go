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
	// QuestionSubjective - 자유 입력 의미 채점 (LLM semantic grading; exact match 불가한 번역/의미 답변용)
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

// Skill defines the skill measured by a question.
// Question.Type remains the answer/rendering mode for backward compatibility.
type Skill string

const (
	// App-owned beginner taxonomy used by current kana/vocabulary seeders.
	SkillKanaReading      Skill = "kana_reading"
	SkillKanaRecall       Skill = "kana_recall"
	SkillKanaHandwriting  Skill = "kana_handwriting"
	SkillVocabMeaning     Skill = "vocab_meaning"
	SkillVocabRecall      Skill = "vocab_recall"
	SkillVocabHandwriting Skill = "vocab_handwriting"

	// JLPT official-style taxonomy.
	SkillKanjiReading           Skill = "kanji_reading"
	SkillVocabContext           Skill = "vocab_context"
	SkillVocabParaphrase        Skill = "vocab_paraphrase"
	SkillVocabUsage             Skill = "vocab_usage"
	SkillGrammarForm            Skill = "grammar_form"
	SkillSentenceComposition    Skill = "sentence_composition"
	SkillTextGrammar            Skill = "text_grammar"
	SkillReadingShort           Skill = "reading_short"
	SkillReadingMid             Skill = "reading_mid"
	SkillReadingLong            Skill = "reading_long"
	SkillReadingIntegrated      Skill = "reading_integrated"
	SkillReadingThematic        Skill = "reading_thematic"
	SkillInformationRetrieval   Skill = "information_retrieval"
	SkillListeningTask          Skill = "listening_task"
	SkillListeningKeyPoint      Skill = "listening_key_point"
	SkillListeningOutline       Skill = "listening_outline"
	SkillListeningQuickResponse Skill = "listening_quick_response"
	SkillListeningIntegrated    Skill = "listening_integrated"
)

func SkillPtr(skill Skill) *Skill {
	return &skill
}

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
	ID               int              `db:"id"                json:"id"`
	ContentID        *int             `db:"content_id"        json:"content_id"`
	Type             QuestionType     `db:"type"              json:"type"`
	Skill            *Skill           `db:"item_type"         json:"item_type,omitempty"`
	Language         string           `db:"language"          json:"language"`          // ISO 639-1: 'ja', 'el', 'en'
	ProficiencyLevel string           `db:"proficiency_level" json:"proficiency_level"` // JLPT: N5-N1, CEFR: A1-C2
	Category         QuestionCategory `db:"category"          json:"category"`
	Prompt           string           `db:"prompt"            json:"prompt"`
	Options          json.RawMessage  `db:"options"           json:"options"`
	CorrectAnswer    string           `db:"correct_answer"    json:"correct_answer"`
	Explanation      string           `db:"explanation"       json:"explanation"`
	AudioPath        *string          `db:"audio_path"        json:"audio_path"`
	Difficulty       int              `db:"difficulty"        json:"difficulty"`
	TimesServed      int              `db:"times_served"      json:"times_served"`
	TimesCorrect     int              `db:"times_correct"     json:"times_correct"`
	// SRS (SM-2) state
	EaseFactor     float64    `db:"ease_factor"      json:"ease_factor"`
	IntervalDays   int        `db:"interval_days"    json:"interval_days"`
	Repetitions    int        `db:"repetitions"      json:"repetitions"`
	NextReviewAt   *time.Time `db:"next_review_at"   json:"next_review_at"`
	LastReviewedAt *time.Time `db:"last_reviewed_at" json:"last_reviewed_at"`
	CreatedAt      time.Time  `db:"created_at"       json:"created_at"`
}

// GetOptions parses the JSONB options field into a string slice.
func (q *Question) GetOptions() ([]string, error) {
	var opts []string
	if err := json.Unmarshal(q.Options, &opts); err != nil {
		return nil, err
	}
	return opts, nil
}
