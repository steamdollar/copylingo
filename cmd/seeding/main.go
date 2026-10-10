// Command seeding upserts a language's seed records into the database
// (ADR-065, ADR-066, ADR-067). Every level is authored as records in
// cmd/seeding/data/<language>/<level>.json, so seeding is one copy path
// regardless of question type.
//
// Usage: go run ./cmd/seeding <language>
package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/jmoiron/sqlx"

	"github.com/lsj/copylingo/internal/bootstrap"
	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/repository"
)

// The build fails unless data/<language>/<level>.json files exist, so data/
// holds only language directories and every embedded path below is readable.
//
//go:embed data/*/*.json
var seedDataFS embed.FS

// levelData is one decoded level file (data/<language>/<level>.json): the
// level's materials with their nested questions, plus material-less questions.
type levelData struct {
	Materials []materialRecord `json:"materials"`
	Questions []model.Question `json:"questions"`
}

type materialRecord struct {
	model.Material
	Questions []model.Question `json:"questions,omitempty"`
}

func main() {
	language, db := setup()
	defer db.Close()

	materials, looseQuestions, err := loadRecords(language)
	if err != nil {
		log.Fatalf(
			"Failed to load %s seed records: %v",
			language,
			err,
		)
	}

	if err := seedRecords(
		context.Background(),
		repository.NewRepositories(db),
		materials,
		looseQuestions,
	); err != nil {
		log.Fatalf(
			"Failed to seed %s records: %v",
			language,
			err,
		)
	}
}

// setup turns the command line and environment into a validated language and
// an open database. A bad argument prints usage and exits 2 before any config
// or database work; config and connection failures are fatal.
func setup() (string, *sqlx.DB) {
	entries, _ := fs.ReadDir(
		seedDataFS,
		"data",
	)
	languages := make(
		[]string,
		0,
		len(entries),
	)
	for _, entry := range entries {
		languages = append(
			languages,
			entry.Name(),
		)
	}
	language, err := parseLanguageArg(
		os.Args[1:],
		languages,
	)
	if err != nil {
		fmt.Fprintf(
			os.Stderr,
			"%v\n\nUsage: go run ./cmd/seeding <language>\nValid languages: %s\n",
			err,
			strings.Join(
				languages,
				", ",
			),
		)
		os.Exit(2)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf(
			"Failed to load config: %v",
			err,
		)
	}
	db, err := bootstrap.OpenDB(cfg.DB)
	if err != nil {
		log.Fatalf(
			"Database connection failed: %v",
			err,
		)
	}
	return language, db
}

// parseLanguageArg takes the one positional argument, which must name an
// embedded data directory exactly, without case or space normalization.
func parseLanguageArg(
	args []string,
	languages []string,
) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf(
			"expected one language argument, got %d",
			len(args),
		)
	}
	language := args[0]
	if !slices.Contains(
		languages,
		language,
	) {
		return "", fmt.Errorf(
			"unknown language %q",
			language,
		)
	}
	return language, nil
}

// loadRecords decodes every level file of a validated language, failing on
// unknown fields so a misspelled key is not silently dropped. Each row gets
// the language and the file's level (data/ja/n5.json -> N5), so adding a
// level is a data change.
func loadRecords(language string) ([]*materialRecord, []*model.Question, error) {
	// The pattern is fixed and the language names an embedded directory.
	levelFilePaths, _ := fs.Glob(
		seedDataFS,
		path.Join(
			"data",
			language,
			"*.json",
		),
	)

	var materials []*materialRecord
	var looseQuestions []*model.Question
	for _, filePath := range levelFilePaths {
		content, _ := seedDataFS.ReadFile(filePath)
		var data levelData
		decoder := json.NewDecoder(bytes.NewReader(content))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&data); err != nil {
			return nil, nil, fmt.Errorf(
				"decode %s: %w",
				filePath,
				err,
			)
		}

		// The pointers reach into data's slices, so the material IDs set by
		// the upsert are visible to collectQuestions.
		level := strings.ToUpper(strings.TrimSuffix(
			path.Base(filePath),
			".json",
		))
		for i := range data.Materials {
			material := &data.Materials[i]
			material.Language = language
			material.ProficiencyLevel = level
			for j := range material.Questions {
				material.Questions[j].Language = language
				material.Questions[j].ProficiencyLevel = level
			}
			materials = append(
				materials,
				material,
			)
		}
		for i := range data.Questions {
			question := &data.Questions[i]
			question.Language = language
			question.ProficiencyLevel = level
			looseQuestions = append(
				looseQuestions,
				question,
			)
		}
	}
	return materials, looseQuestions, nil
}

// seedRecords upserts every material first, because UpsertBatch sets the
// material IDs that nested questions link to, then every question.
func seedRecords(
	ctx context.Context,
	repos *repository.Repositories,
	materials []*materialRecord,
	looseQuestions []*model.Question,
) error {
	materialRows := make(
		[]*model.Material,
		len(materials),
	)
	for i, material := range materials {
		materialRows[i] = &material.Material
	}
	if err := repos.Material.UpsertBatch(
		ctx,
		materialRows,
	); err != nil {
		return fmt.Errorf(
			"upsert materials batch: %w",
			err,
		)
	}
	log.Printf(
		"Successfully upserted %d materials.",
		len(materialRows),
	)

	questions := collectQuestions(
		materials,
		looseQuestions,
	)
	if err := repos.Question.UpsertSeedBatch(
		ctx,
		questions,
	); err != nil {
		return fmt.Errorf(
			"upsert questions batch: %w",
			err,
		)
	}
	log.Printf(
		"Successfully upserted %d questions.",
		len(questions),
	)
	return nil
}

// collectQuestions links each nested question to its material's upserted row
// and appends the material-less questions.
func collectQuestions(
	materials []*materialRecord,
	looseQuestions []*model.Question,
) []*model.Question {
	var questions []*model.Question
	for _, material := range materials {
		for i := range material.Questions {
			question := &material.Questions[i]
			question.MaterialID = &material.ID
			questions = append(
				questions,
				question,
			)
		}
	}
	return append(
		questions,
		looseQuestions...,
	)
}
