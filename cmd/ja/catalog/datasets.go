package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/lsj/copylingo/internal/model"
)

// The JSON files under data/ are the content source of truth; question- and
// material-generation logic stays in Go. Edit the JSON to change content.

//go:embed data/kana.json
var kanaJSON []byte

//go:embed data/n5_vocab.json
var n5VocabJSON []byte

//go:embed data/n5_grammar.json
var n5GrammarJSON []byte

//go:embed data/n5_vocab_context.json
var n5VocabContextJSON []byte

//go:embed data/n5_listening.json
var n5ListeningJSON []byte

const (
	VocabLanguage         = "ja"
	VocabProficiencyLevel = "N5"
	VocabDifficulty       = 2

	GrammarDifficulty = 2
)

type VocabWord struct {
	ID           string `json:"id"`
	Kana         string `json:"kana"`
	Kanji        string `json:"kanji"`
	MeaningKo    string `json:"meaning_ko"`
	PartOfSpeech string `json:"part_of_speech"`
}

type GrammarPoint struct {
	ID            string `json:"id"`
	Pattern       string `json:"pattern"`
	MeaningKo     string `json:"meaning_ko"`
	ExplanationKo string `json:"explanation_ko"`
	Example       string `json:"example"`
	// ExampleReading is the full-hiragana reading of Example (katakana kept as-is),
	// so learners can read kanji-heavy example sentences. Seeded into the grammar
	// material payload and shown as a 읽기 line under the 예문.
	ExampleReading string   `json:"example_reading"`
	TranslationKo  string   `json:"translation_ko"`
	ClozePrompt    string   `json:"cloze_prompt"`
	CorrectAnswer  string   `json:"correct_answer"`
	FormOptions    []string `json:"form_options"`
}

// VocabContext carries the cloze data for a single word's 文脈規定 questions.
// WordID references an existing N5Words entry; coverage is partial by design
// (only words with authored example sentences get context questions). Each
// cloze in Clozes becomes one static question sharing FormOptions/CorrectAnswer.
type VocabContext struct {
	WordID        string   `json:"word_id"`
	CorrectAnswer string   `json:"correct_answer"`
	FormOptions   []string `json:"form_options"`
	Clozes        []string `json:"clozes"`
}

// ListeningQuestion is an original N5 listening-comprehension MCQ. Script is
// synthesized into audio and is intentionally separate from the visible prompt.
type ListeningQuestion struct {
	ID            string      `json:"id"`
	Skill         model.Skill `json:"skill"`
	Script        string      `json:"script"`
	Prompt        string      `json:"prompt"`
	Options       []string    `json:"options"`
	CorrectAnswer string      `json:"correct_answer"`
	Explanation   string      `json:"explanation"`
	Difficulty    int         `json:"difficulty"`
}

// KanaMap maps each kana to its romaji. Script-label and hint logic lives in Go.
var KanaMap = mustLoadJSON[map[string]string]("kana", kanaJSON)

// N5Words is the N5 vocabulary catalog.
var N5Words = mustLoadJSON[[]VocabWord]("n5_vocab", n5VocabJSON)

// N5GrammarPoints is the N5 grammar catalog.
var N5GrammarPoints = mustLoadJSON[[]GrammarPoint]("n5_grammar", n5GrammarJSON)

// N5VocabContext is the N5 vocabulary 文脈規定 catalog (partial coverage).
var N5VocabContext = mustLoadJSON[[]VocabContext]("n5_vocab_context", n5VocabContextJSON)

// N5ListeningQuestions is the original N5 listening-comprehension catalog.
var N5ListeningQuestions = mustLoadJSON[[]ListeningQuestion]("n5_listening", n5ListeningJSON)

// loadJSON decodes an embedded dataset into T.
func loadJSON[T any](name string, data []byte) (T, error) {
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return v, fmt.Errorf("catalog: load %s dataset: %w", name, err)
	}
	return v, nil
}

// mustLoadJSON decodes an embedded dataset at package init time. A failure means
// the embedded JSON is malformed — a build/data defect, not a runtime condition —
// so panicking surfaces it immediately.
func mustLoadJSON[T any](name string, data []byte) T {
	v, err := loadJSON[T](name, data)
	if err != nil {
		panic(err)
	}
	return v
}
