package catalog

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lsj/copylingo/internal/model"
)

// The JSON files under data/ are the content source of truth; question- and
// material-generation logic stays in Go. Edit the JSON to change content.

//go:embed data/*.json
var dataFS embed.FS

const (
	VocabLanguage   = "ja"
	VocabDifficulty = 2

	GrammarDifficulty = 2
)

type VocabWord struct {
	ID           string `json:"id"`
	Level        string `json:"level,omitempty"`
	Kana         string `json:"kana"`
	Kanji        string `json:"kanji"`
	MeaningKo    string `json:"meaning_ko"`
	PartOfSpeech string `json:"part_of_speech"`
}

type GrammarPoint struct {
	ID            string `json:"id"`
	Level         string `json:"level,omitempty"`
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
// WordID references an existing word in the same level catalog; coverage is partial by design
// (only words with authored example sentences get context questions). Each
// cloze in Clozes becomes one static question sharing FormOptions/CorrectAnswer.
type VocabContext struct {
	WordID        string   `json:"word_id"`
	CorrectAnswer string   `json:"correct_answer"`
	FormOptions   []string `json:"form_options"`
	Clozes        []string `json:"clozes"`
}

// ListeningQuestion is an original listening-comprehension MCQ. Script is
// synthesized into audio and is intentionally separate from the visible prompt.
type ListeningQuestion struct {
	ID            string      `json:"id"`
	Level         string      `json:"level,omitempty"`
	Skill         model.Skill `json:"skill"`
	Script        string      `json:"script"`
	Prompt        string      `json:"prompt"`
	Options       []string    `json:"options"`
	CorrectAnswer string      `json:"correct_answer"`
	Explanation   string      `json:"explanation"`
	AudioPath     string      `json:"audio_path,omitempty"`
	Difficulty    int         `json:"difficulty"`
}

// ReadingVocabulary is one key-vocabulary entry surfaced on a reading study card.
type ReadingVocabulary struct {
	Surface   string `json:"surface"`
	Reading   string `json:"reading"`
	MeaningKo string `json:"meaning_ko"`
}

// ReadingPassage is an original reading passage plus one MCQ over it.
// Passage/Reading/KeyVocabulary feed the study material; Prompt/Options/
// CorrectAnswer/Explanation feed the quiz question (ADR-036).
type ReadingPassage struct {
	ID            string              `json:"id"`
	Level         string              `json:"level,omitempty"`
	Skill         model.Skill         `json:"skill"`
	Title         string              `json:"title"`
	Passage       string              `json:"passage"`
	Reading       string              `json:"reading"`
	KeyVocabulary []ReadingVocabulary `json:"key_vocabulary"`
	Prompt        string              `json:"prompt"`
	Options       []string            `json:"options"`
	CorrectAnswer string              `json:"correct_answer"`
	Explanation   string              `json:"explanation"`
	Difficulty    int                 `json:"difficulty"`
}

// WordOrderQuestion is a static sentence-composition item. Chunks retain
// their authored order in the catalog; the Telegram renderer shuffles them
// deterministically per session/question while callbacks carry the original
// option index.
type WordOrderQuestion struct {
	ID            string   `json:"id"`
	GrammarID     string   `json:"grammar_id"`
	Prompt        string   `json:"prompt"`
	Chunks        []string `json:"chunks"`
	CorrectAnswer string   `json:"correct_answer"`
	Explanation   string   `json:"explanation"`
}

// QuestionSeed is an authored question that does not need a specialized
// generator. SourceID (or ID for legacy fixtures) is used to derive a stable
// level-aware question key. MaterialKey may link vocabulary, grammar, or
// reading questions to their study material. Listening seeds intentionally do
// not carry a material link; AudioScript and AudioPath feed the audio pipeline.
type QuestionSeed struct {
	ID            string                 `json:"id,omitempty"`
	SourceID      string                 `json:"source_id,omitempty"`
	ItemType      model.Skill            `json:"item_type"`
	Type          model.QuestionType     `json:"type"`
	Category      model.QuestionCategory `json:"category"`
	MaterialKey   string                 `json:"material_key,omitempty"`
	Prompt        string                 `json:"prompt"`
	Options       []string               `json:"options"`
	CorrectAnswer string                 `json:"correct_answer"`
	Explanation   string                 `json:"explanation"`
	AudioScript   string                 `json:"audio_script,omitempty"`
	AudioPath     string                 `json:"audio_path,omitempty"`
	Difficulty    int                    `json:"difficulty"`
}

// LevelCatalog groups every authored dataset by proficiency level. Adding a
// level extends the registry instead of adding level-named Go variables and
// branching throughout material or question assembly.
type LevelCatalog struct {
	Level                       string
	Words                       []VocabWord
	GrammarPoints               []GrammarPoint
	VocabContexts               []VocabContext
	ListeningQuestions          []ListeningQuestion
	ReadingPassages             []ReadingPassage
	WordOrderQuestions          []WordOrderQuestion
	QuestionSeeds               []QuestionSeed
	GenerateVocabularyQuestions bool
	GenerateGrammarQuestions    bool
}

type levelCatalogFiles struct {
	level                       string
	vocab                       string
	grammar                     string
	vocabContext                string
	listening                   string
	reading                     string
	wordOrder                   string
	questionSeeds               string
	generateVocabularyQuestions bool
	generateGrammarQuestions    bool
}

var catalogFiles = []levelCatalogFiles{
	{
		level: "N5", vocab: "n5_vocab.json", grammar: "n5_grammar.json",
		vocabContext: "n5_vocab_context.json", listening: "n5_listening.json",
		reading: "n5_reading.json", wordOrder: "n5_word_order.json",
		generateVocabularyQuestions: true, generateGrammarQuestions: true,
	},
	{
		level: "N4", vocab: "n4_vocab.json", grammar: "n4_grammar.json",
		listening: "n4_listening.json", reading: "n4_reading.json",
		questionSeeds: "n4_question_seeds.json",
	},
}

// KanaMap maps each kana to its romaji. Script-label and hint logic lives in Go.
var KanaMap = mustLoadJSONFile[map[string]string]("kana.json")

var levelCatalogs = loadLevelCatalogs(catalogFiles)

// LevelCatalogs returns the registered catalogs in seeding order.
func LevelCatalogs() []LevelCatalog {
	return append([]LevelCatalog(nil), levelCatalogs...)
}

// LevelCatalogFor resolves a catalog without exposing level-specific symbols.
func LevelCatalogFor(level string) (LevelCatalog, bool) {
	level = strings.ToUpper(strings.TrimSpace(level))
	for _, catalog := range levelCatalogs {
		if catalog.Level == level {
			return catalog, true
		}
	}
	return LevelCatalog{}, false
}

// DefaultProficiencyLevel is the level used by legacy single-level builders
// and kana content. The first registry entry owns that compatibility policy.
func DefaultProficiencyLevel() string {
	if len(levelCatalogs) == 0 {
		panic("catalog: no level catalogs registered")
	}
	return levelCatalogs[0].Level
}

func loadLevelCatalogs(files []levelCatalogFiles) []LevelCatalog {
	catalogs := make([]LevelCatalog, 0, len(files))
	for _, file := range files {
		catalogs = append(catalogs, LevelCatalog{
			Level:                       strings.ToUpper(strings.TrimSpace(file.level)),
			Words:                       loadOptionalJSONFile[[]VocabWord](file.vocab),
			GrammarPoints:               loadOptionalJSONFile[[]GrammarPoint](file.grammar),
			VocabContexts:               loadOptionalJSONFile[[]VocabContext](file.vocabContext),
			ListeningQuestions:          loadOptionalJSONFile[[]ListeningQuestion](file.listening),
			ReadingPassages:             loadOptionalJSONFile[[]ReadingPassage](file.reading),
			WordOrderQuestions:          loadOptionalJSONFile[[]WordOrderQuestion](file.wordOrder),
			QuestionSeeds:               loadOptionalJSONFile[[]QuestionSeed](file.questionSeeds),
			GenerateVocabularyQuestions: file.generateVocabularyQuestions,
			GenerateGrammarQuestions:    file.generateGrammarQuestions,
		})
	}
	return catalogs
}

func loadOptionalJSONFile[T any](name string) T {
	if name == "" {
		var zero T
		return zero
	}
	return mustLoadJSONFile[T](name)
}

func mustLoadJSONFile[T any](name string) T {
	data, err := dataFS.ReadFile("data/" + name)
	if err != nil {
		panic(fmt.Errorf("catalog: read %s dataset: %w", name, err))
	}
	return mustLoadJSON[T](name, data)
}

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
