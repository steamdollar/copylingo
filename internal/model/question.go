package model

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math/rand"
	"time"
)

// QuestionType defines the type of question.
type QuestionType string

const (
	// QuestionMultipleChoice - 객관식 (4지선다 등)
	QuestionMultipleChoice QuestionType = "multiple_choice"
	// QuestionFillBlank - 빈칸 채우기
	QuestionFillBlank QuestionType = "fill_blank"
	// QuestionKanaHandwriting - 가나 손글씨 입력 (Mini App + AI binary grading)
	QuestionKanaHandwriting QuestionType = "kana_handwriting"
	// QuestionTranslation - 번역 문제
	QuestionTranslation QuestionType = "translation"
	// QuestionListening - 청해 문제
	QuestionListening QuestionType = "listening"
	// QuestionReadingComprehension - 독해 문제
	QuestionReadingComprehension QuestionType = "reading_comp"
	// QuestionWordOrder - 단어 배열 (어순 교정)
	QuestionWordOrder QuestionType = "word_order"
)

// Skill defines the skill measured by a question.
// Question.Type remains the answer/rendering mode for backward compatibility.
type Skill string

const (
	// App-owned beginner taxonomy used by current kana/vocabulary seeders.
	SkillKanaReading           Skill = "kana_reading"
	SkillKanaRecall            Skill = "kana_recall"
	SkillKanaHandwriting       Skill = "kana_handwriting"
	SkillVocabularyMeaning     Skill = "vocab_meaning"
	SkillVocabularyRecall      Skill = "vocab_recall"
	SkillVocabularyKanjiRecall Skill = "vocab_kanji_recall"
	SkillVocabularyHandwriting Skill = "vocab_handwriting"

	// JLPT official-style taxonomy.
	SkillKanjiReading Skill = "kanji_reading"
	// SkillVocabularyKanjiReading is the N4/JLPT vocabulary kanji-reading item type.
	// It is intentionally distinct from the legacy app-owned vocab_kanji_recall.
	SkillVocabularyKanjiReading Skill = "vocab_kanji_reading"
	SkillVocabularyOrthography  Skill = "vocab_orthography"
	SkillVocabularyContext      Skill = "vocab_context"
	SkillVocabularyParaphrase   Skill = "vocab_paraphrase"
	SkillVocabularyUsage        Skill = "vocab_usage"
	SkillGrammarForm            Skill = "grammar_form"
	SkillSentenceComposition    Skill = "sentence_composition"
	SkillGrammarText            Skill = "grammar_text"
	SkillTextGrammar            Skill = "text_grammar"
	SkillReadingShort           Skill = "reading_short"
	SkillReadingMedium          Skill = "reading_medium"
	SkillReadingMid             Skill = "reading_mid"
	SkillReadingLong            Skill = "reading_long"
	SkillReadingIntegrated      Skill = "reading_integrated"
	SkillReadingThematic        Skill = "reading_thematic"
	SkillReadingInformation     Skill = "reading_information"
	SkillInformationRetrieval   Skill = "information_retrieval"
	SkillListeningTask          Skill = "listening_task"
	SkillListeningKeyPoint      Skill = "listening_key_point"
	SkillListeningVerbal        Skill = "listening_verbal"
	SkillListeningOutline       Skill = "listening_outline"
	SkillListeningQuickResponse Skill = "listening_quick_response"
	SkillListeningIntegrated    Skill = "listening_integrated"
)

func SkillPointer(skill Skill) *Skill {
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

// Question represents a reusable learning-question catalog entry.
type Question struct {
	ID               int              `db:"id"                json:"id"`
	QuestionKey      *string          `db:"question_key"      json:"question_key,omitempty"`
	ContentID        *int             `db:"content_id"        json:"content_id"`
	MaterialID       *int             `db:"material_id"       json:"material_id,omitempty"`
	Type             QuestionType     `db:"type"              json:"type"`
	Skill            *Skill           `db:"item_type"         json:"item_type,omitempty"`
	Language         string           `db:"language"          json:"language"`          // ISO 639-1: 'ja', 'el', 'en'
	ProficiencyLevel string           `db:"proficiency_level" json:"proficiency_level"` // JLPT: N5-N1, CEFR: A1-C2
	Category         QuestionCategory `db:"category"          json:"category"`
	Prompt           string           `db:"prompt"            json:"prompt"`
	Options          json.RawMessage  `db:"options"           json:"options"`
	CorrectAnswer    string           `db:"correct_answer"    json:"correct_answer"`
	Explanation      string           `db:"explanation"       json:"explanation"`
	AudioPath        *string          `db:"audio_path"        json:"audio_path"`              // object key in the S3-compatible store (ADR-032)
	AudioScript      *string          `db:"audio_script"      json:"audio_script,omitempty"`  // listening: text synthesized into audio (ADR-031)
	AudioFileID      *string          `db:"audio_file_id"     json:"audio_file_id,omitempty"` // listening: cached Telegram file_id (ADR-032)
	Difficulty       int              `db:"difficulty"        json:"difficulty"`
	CreatedAt        time.Time        `db:"created_at"        json:"created_at"`
	// IsMaintenanceCheck is a transient, user-scoped selection hint. It is never
	// written to the shared catalog or serialized into session/cache payloads.
	IsMaintenanceCheck bool `db:"is_maintenance_check" json:"-"`
}

// GetOptions parses the JSONB options field into a string slice.
func (question *Question) GetOptions() ([]string, error) {
	var options []string
	if err := json.Unmarshal(
		question.Options,
		&options,
	); err != nil {
		return nil, err
	}
	return options, nil
}

// ShuffleOptions deterministically shuffles the question options using a seed derived from sessionID and Question.ID.
// If options contains 1 or fewer elements, or is empty/nil, it does nothing and returns nil.
func (question *Question) ShuffleOptions(sessionID int) error {
	if len(question.Options) == 0 {
		return nil
	}
	var options []string
	if err := json.Unmarshal(
		question.Options,
		&options,
	); err != nil {
		return err
	}
	if len(options) <= 1 {
		return nil
	}

	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(fmt.Sprintf(
		"%d:%d",
		sessionID,
		question.ID,
	)))
	random := rand.New(rand.NewSource(int64(hasher.Sum64())))
	random.Shuffle(
		len(options),
		func(
			i,
			j int,
		) {
			options[i], options[j] = options[j], options[i]
		},
	)

	shuffledJSON, err := json.Marshal(options)
	if err != nil {
		return err
	}
	question.Options = shuffledJSON
	return nil
}
