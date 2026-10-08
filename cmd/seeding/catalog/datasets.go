package catalog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lsj/copylingo/cmd/seeding/data"
	"github.com/lsj/copylingo/internal/model"
)

// One JSON file per level, cmd/seeding/data/<language>/<level>.json, is the
// content source of truth (ADR-066, ADR-067). The seeder only copies it into
// the database; nothing is generated at seed time.

// Japanese is the only seeded language today. The registry, data layout, and
// seeder CLI are keyed by language (ADR-065).
const Japanese = "ja"

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

// levelFile is the shape of every level JSON file.
type levelFile struct {
	Materials []MaterialRecord `json:"materials"`
	Questions []QuestionRecord `json:"questions"`
}

// LevelCatalog is one registered language level and its records. Materials
// carry their nested questions; Questions holds the material-less ones.
type LevelCatalog struct {
	Language  string
	Level     string
	Materials []MaterialRecord
	Questions []QuestionRecord
}

// levelCatalogFiles registers one level and its file under data/<language>/.
type levelCatalogFiles struct {
	language string
	level    string
	file     string
}

var catalogFiles = []levelCatalogFiles{
	{language: Japanese, level: "N5", file: "n5.json"},
	{language: Japanese, level: "N4", file: "n4.json"},
}

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

// DefaultProficiencyLevel is the default level for single-level tools. Each
// language's first registry entry owns that policy.
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
		records := mustLoadLevelFile(
			language,
			file.file,
		)
		catalogs = append(
			catalogs,
			LevelCatalog{
				Language:  language,
				Level:     strings.ToUpper(strings.TrimSpace(file.level)),
				Materials: records.Materials,
				Questions: records.Questions,
			},
		)
	}
	return catalogs
}

// mustLoadLevelFile decodes data/<language>/<name> at package init time.
// Unknown fields are rejected so a misspelled key (e.g. an optional
// audio_script) fails loudly instead of being dropped. A failure is a data
// defect, not a runtime condition, so it panics.
func mustLoadLevelFile(
	language,
	name string,
) levelFile {
	path := language + "/" + name
	content, err := data.FS.ReadFile(path)
	if err != nil {
		panic(fmt.Errorf(
			"catalog: read %s dataset: %w",
			path,
			err,
		))
	}
	var file levelFile
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&file); err != nil {
		panic(fmt.Errorf(
			"catalog: decode %s dataset: %w",
			path,
			err,
		))
	}
	return file
}
