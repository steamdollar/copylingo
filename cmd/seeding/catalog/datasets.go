package catalog

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"

	"github.com/lsj/copylingo/cmd/seeding/data"
	"github.com/lsj/copylingo/internal/model"
)

// The JSON files under cmd/seeding/data/<language>/ are the content source of
// truth; question- and material-generation logic stays in Go. Edit the JSON to
// change content.

// Japanese is the only seeded language today. The registry, data layout, and
// seeder CLI are keyed by language, but the material builders here are still
// Japanese-shaped (kana, kanji), so they stamp this code directly (ADR-065).
const Japanese = "ja"

const (
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

// MaterialRecord is one materials-table row as authored data (ADR-066),
// with the questions that link to it nested underneath. Nesting mirrors the
// questions.material_id foreign key, so a question cannot point at a
// material that is not seeded. Language and level come from the record's
// data/<language>/<level>/ directory. Payload is opaque to the seeder; its
// shape is the contract between the category and the bot's study-card
// renderer.
type MaterialRecord struct {
	MaterialKey string                 `json:"material_key"`
	Category    model.MaterialCategory `json:"category"`
	Title       string                 `json:"title"`
	Difficulty  int                    `json:"difficulty"`
	Payload     json.RawMessage        `json:"payload"`
	Questions   []QuestionRecord       `json:"questions,omitempty"`
}

// QuestionRecord is one questions-table row as authored data (ADR-066).
// QuestionKey is the upsert identity and is written verbatim, so converted
// questions keep the key (and therefore the row and learner progress) they
// had under their former type-specific builders. A record nested under a
// material links to it; a top-level record has no material (listening plays
// AudioScript instead).
type QuestionRecord struct {
	QuestionKey   string                 `json:"question_key"`
	ItemType      model.Skill            `json:"item_type"`
	Type          model.QuestionType     `json:"type"`
	Category      model.QuestionCategory `json:"category"`
	Prompt        string                 `json:"prompt"`
	Options       []string               `json:"options"`
	CorrectAnswer string                 `json:"correct_answer"`
	Explanation   string                 `json:"explanation"`
	AudioScript   string                 `json:"audio_script,omitempty"`
	Difficulty    int                    `json:"difficulty"`
}

// levelRecords is the shape of every JSON file in a record level directory.
type levelRecords struct {
	Materials []MaterialRecord `json:"materials"`
	Questions []QuestionRecord `json:"questions"`
}

// LevelCatalog groups every authored dataset by language and proficiency
// level. Adding a level extends the registry instead of adding level-named Go
// variables and branching throughout material or question assembly.
//
// Materials (with nested questions) and Questions (material-less) hold
// levels converted to the unified record format; the typed fields above them
// are the legacy N5 datasets that still go through type-specific builders
// until they are converted.
type LevelCatalog struct {
	Language                    string
	Level                       string
	Words                       []VocabWord
	GrammarPoints               []GrammarPoint
	VocabContexts               []VocabContext
	ListeningQuestions          []ListeningQuestion
	ReadingPassages             []ReadingPassage
	WordOrderQuestions          []WordOrderQuestion
	Materials                   []MaterialRecord
	Questions                   []QuestionRecord
	GenerateVocabularyQuestions bool
	GenerateGrammarQuestions    bool
}

// levelCatalogFiles names one level's datasets. Legacy file names resolve
// under data/<language>/; recordDir names data/<language>/<recordDir>/,
// whose JSON files hold unified records.
type levelCatalogFiles struct {
	language                    string
	level                       string
	vocab                       string
	grammar                     string
	vocabContext                string
	listening                   string
	reading                     string
	wordOrder                   string
	recordDir                   string
	generateVocabularyQuestions bool
	generateGrammarQuestions    bool
}

var catalogFiles = []levelCatalogFiles{
	{
		language: Japanese, level: "N5", vocab: "n5_vocab.json", grammar: "n5_grammar.json",
		vocabContext: "n5_vocab_context.json", listening: "n5_listening.json",
		reading: "n5_reading.json", wordOrder: "n5_word_order.json",
		generateVocabularyQuestions: true, generateGrammarQuestions: true,
	},
	{language: Japanese, level: "N4", recordDir: "n4"},
}

// KanaMap maps each kana to its romaji. Script-label and hint logic lives in Go.
var KanaMap = mustLoadJSONFile[map[string]string](
	Japanese,
	"kana.json",
)

var levelCatalogs = loadLevelCatalogs(catalogFiles)

// LevelCatalogsFor returns a language's registered catalogs in seeding order.
// An unregistered language yields an empty slice.
func LevelCatalogsFor(language string) []LevelCatalog {
	language = normalizeLanguage(language)
	catalogs := make(
		[]LevelCatalog,
		0,
		len(levelCatalogs),
	)
	for _, catalog := range levelCatalogs {
		if catalog.Language == language {
			catalogs = append(
				catalogs,
				catalog,
			)
		}
	}
	return catalogs
}

// LevelCatalogFor resolves a catalog without exposing level-specific symbols.
func LevelCatalogFor(
	language,
	level string,
) (LevelCatalog, bool) {
	language = normalizeLanguage(language)
	level = strings.ToUpper(strings.TrimSpace(level))
	for _, catalog := range levelCatalogs {
		if catalog.Language == language && catalog.Level == level {
			return catalog, true
		}
	}
	return LevelCatalog{}, false
}

// DefaultProficiencyLevel is the level used by legacy single-level builders
// and kana content. Each language's first registry entry owns that policy.
func DefaultProficiencyLevel(language string) string {
	catalogs := LevelCatalogsFor(language)
	if len(catalogs) == 0 {
		panic(fmt.Sprintf(
			"catalog: no level catalogs registered for language %q",
			language,
		))
	}
	return catalogs[0].Level
}

func normalizeLanguage(language string) string {
	return strings.ToLower(strings.TrimSpace(language))
}

func loadLevelCatalogs(files []levelCatalogFiles) []LevelCatalog {
	catalogs := make(
		[]LevelCatalog,
		0,
		len(files),
	)
	for _, file := range files {
		language := normalizeLanguage(file.language)
		records := loadRecordDir(
			language,
			file.recordDir,
		)
		catalogs = append(
			catalogs,
			LevelCatalog{
				Language: language,
				Level:    strings.ToUpper(strings.TrimSpace(file.level)),
				Words: loadOptionalJSONFile[[]VocabWord](
					language,
					file.vocab,
				),
				GrammarPoints: loadOptionalJSONFile[[]GrammarPoint](
					language,
					file.grammar,
				),
				VocabContexts: loadOptionalJSONFile[[]VocabContext](
					language,
					file.vocabContext,
				),
				ListeningQuestions: loadOptionalJSONFile[[]ListeningQuestion](
					language,
					file.listening,
				),
				ReadingPassages: loadOptionalJSONFile[[]ReadingPassage](
					language,
					file.reading,
				),
				WordOrderQuestions: loadOptionalJSONFile[[]WordOrderQuestion](
					language,
					file.wordOrder,
				),
				Materials:                   records.Materials,
				Questions:                   records.Questions,
				GenerateVocabularyQuestions: file.generateVocabularyQuestions,
				GenerateGrammarQuestions:    file.generateGrammarQuestions,
			},
		)
	}
	return catalogs
}

func loadOptionalJSONFile[T any](
	language,
	name string,
) T {
	if name == "" {
		var zero T
		return zero
	}
	return mustLoadJSONFile[T](
		language,
		name,
	)
}

// loadRecordDir merges every JSON file under data/<language>/<dir>/ in
// file-name order. Each file has the same {materials, questions} shape, so a
// level can live in one file or be split by count without code changes.
func loadRecordDir(
	language,
	dir string,
) levelRecords {
	var records levelRecords
	if dir == "" {
		return records
	}
	pattern := language + "/" + dir + "/*.json"
	names, err := fs.Glob(
		data.FS,
		pattern,
	)
	if err != nil {
		panic(fmt.Errorf(
			"catalog: glob %s datasets: %w",
			pattern,
			err,
		))
	}
	for _, name := range names {
		content, err := data.FS.ReadFile(name)
		if err != nil {
			panic(fmt.Errorf(
				"catalog: read %s dataset: %w",
				name,
				err,
			))
		}
		file := mustLoadJSON[levelRecords](
			name,
			content,
		)
		records.Materials = append(
			records.Materials,
			file.Materials...,
		)
		records.Questions = append(
			records.Questions,
			file.Questions...,
		)
	}
	return records
}

func mustLoadJSONFile[T any](
	language,
	name string,
) T {
	path := language + "/" + name
	content, err := data.FS.ReadFile(path)
	if err != nil {
		panic(fmt.Errorf(
			"catalog: read %s dataset: %w",
			path,
			err,
		))
	}
	return mustLoadJSON[T](
		path,
		content,
	)
}

// loadJSON decodes an embedded dataset into T.
func loadJSON[T any](
	name string,
	data []byte,
) (T, error) {
	var v T
	if err := json.Unmarshal(
		data,
		&v,
	); err != nil {
		return v, fmt.Errorf(
			"catalog: load %s dataset: %w",
			name,
			err,
		)
	}
	return v, nil
}

// mustLoadJSON decodes an embedded dataset at package init time. A failure means
// the embedded JSON is malformed — a build/data defect, not a runtime condition —
// so panicking surfaces it immediately.
func mustLoadJSON[T any](
	name string,
	data []byte,
) T {
	v, err := loadJSON[T](
		name,
		data,
	)
	if err != nil {
		panic(err)
	}
	return v
}
