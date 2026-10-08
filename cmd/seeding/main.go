// Command seeding upserts a language's seed records into the database
// (ADR-065, ADR-066, ADR-067). Every level is authored as records in
// cmd/seeding/data/<language>/<level>.json, so seeding is one copy path
// regardless of question type.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"github.com/lsj/copylingo/cmd/seeding/catalog"
	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/repository"
)

type levelCatalog = catalog.LevelCatalog

func initDB(cfg *config.Config) (*sqlx.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.DB.Host,
		cfg.DB.Port,
		cfg.DB.User,
		cfg.DB.Password,
		cfg.DB.DBName,
		cfg.DB.SSLMode,
	)
	db, err := sqlx.Connect(
		"postgres",
		dsn,
	)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func main() {
	language := flag.String(
		"language",
		catalog.Japanese,
		"seed catalog language (ISO 639-1); datasets live under cmd/seeding/data/<language>/",
	)
	flag.Parse()

	levelCatalogs := catalog.LevelCatalogsFor(*language)
	if len(levelCatalogs) == 0 {
		log.Fatalf(
			"No seed catalog registered for language %q",
			*language,
		)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf(
			"Failed to load config: %v",
			err,
		)
	}

	db, err := initDB(cfg)
	if err != nil {
		log.Fatalf(
			"Database connection failed: %v",
			err,
		)
	}
	defer db.Close()

	repos := repository.NewRepositories(db)
	ctx := context.Background()

	var materials []*model.Material
	for _, entry := range levelCatalogs {
		materials = append(
			materials,
			catalog.BuildRecordMaterials(
				entry.Language,
				entry.Level,
				entry.Materials,
			)...,
		)
	}
	if err := repos.Material.UpsertBatch(
		ctx,
		materials,
	); err != nil {
		log.Fatalf(
			"Failed to upsert %s materials batch: %v",
			*language,
			err,
		)
	}
	log.Printf(
		"Successfully upserted %d %s materials.",
		len(materials),
		*language,
	)

	recordMaterialIDs, err := loadRecordMaterialIDs(
		ctx,
		repos.Material,
		levelCatalogs,
	)
	if err != nil {
		log.Fatalf(
			"Failed to load record materials: %v",
			err,
		)
	}

	var questions []*model.Question
	for _, entry := range levelCatalogs {
		recordQuestions, err := buildRecordQuestions(
			entry,
			recordMaterialIDs,
		)
		if err != nil {
			log.Fatalf(
				"Failed to build %s %s record questions: %v",
				entry.Language,
				entry.Level,
				err,
			)
		}
		questions = append(
			questions,
			recordQuestions...,
		)
	}

	if err := repos.Question.UpsertSeedBatch(
		ctx,
		questions,
	); err != nil {
		log.Printf(
			"Failed to upsert %s questions batch: %v",
			*language,
			err,
		)
		return
	}

	log.Printf(
		"Successfully upserted %d %s questions across %d proficiency levels.",
		len(questions),
		*language,
		len(levelCatalogs),
	)
}

// materialKeyStore resolves upserted materials by their stable key.
type materialKeyStore interface {
	GetByMaterialKeys(
		ctx context.Context,
		keys []string,
	) ([]model.Material, error)
}

// loadRecordMaterialIDs resolves the upserted row ID of every record material
// that has nested questions, in one query. Unresolved keys are reported per
// question by buildRecordQuestions.
func loadRecordMaterialIDs(
	ctx context.Context,
	store materialKeyStore,
	catalogs []levelCatalog,
) (map[string]int, error) {
	keys := make(
		[]string,
		0,
	)
	for _, entry := range catalogs {
		for _, material := range entry.Materials {
			if len(material.Questions) > 0 {
				keys = append(
					keys,
					material.MaterialKey,
				)
			}
		}
	}
	idsByKey := make(
		map[string]int,
		len(keys),
	)
	if len(keys) == 0 {
		return idsByKey, nil
	}
	materials, err := store.GetByMaterialKeys(
		ctx,
		keys,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"get record materials by key: %w",
			err,
		)
	}
	for _, material := range materials {
		idsByKey[material.MaterialKey] = material.ID
	}
	return idsByKey, nil
}

// buildRecordQuestions maps a level's unified question records to rows:
// questions nested under a material link to it, top-level questions stay
// material-less. Every item type takes this one path. The checks guard what
// the database and the bot rely on: a unique upsert key, the fields every
// renderer reads, and the listening/material split (listening plays audio
// instead of a material).
func buildRecordQuestions(
	entry levelCatalog,
	materialIDsByKey map[string]int,
) ([]*model.Question, error) {
	// Pair each record with its material first so one loop validates and
	// maps nested and top-level records alike.
	type linkedRecord struct {
		record     catalog.QuestionRecord
		materialID *int
	}
	linked := make(
		[]linkedRecord,
		0,
		len(entry.Questions),
	)
	for _, material := range entry.Materials {
		if len(material.Questions) == 0 {
			continue
		}
		materialID, ok := materialIDsByKey[material.MaterialKey]
		if !ok {
			return nil, fmt.Errorf(
				"material %q not found for its nested questions",
				material.MaterialKey,
			)
		}
		for _, record := range material.Questions {
			if record.Category == model.CategoryListening {
				return nil, fmt.Errorf(
					"listening question record %q cannot belong to material %q",
					record.QuestionKey,
					material.MaterialKey,
				)
			}
			linked = append(
				linked,
				linkedRecord{
					record:     record,
					materialID: &materialID,
				},
			)
		}
	}
	for _, record := range entry.Questions {
		linked = append(
			linked,
			linkedRecord{record: record},
		)
	}

	questions := make(
		[]*model.Question,
		0,
		len(linked),
	)
	seenKeys := make(
		map[string]struct{},
		len(linked),
	)
	for _, item := range linked {
		record := item.record
		if record.QuestionKey == "" || record.ItemType == "" || record.Type == "" || record.Category == "" ||
			record.Prompt == "" ||
			record.CorrectAnswer == "" ||
			record.Difficulty < 1 {
			return nil, fmt.Errorf(
				"question record %q: missing required field",
				record.QuestionKey,
			)
		}
		if _, exists := seenKeys[record.QuestionKey]; exists {
			return nil, fmt.Errorf(
				"question record %q: duplicate question key",
				record.QuestionKey,
			)
		}
		seenKeys[record.QuestionKey] = struct{}{}
		question := &model.Question{
			Type:             record.Type,
			Skill:            model.SkillPtr(record.ItemType),
			Language:         entry.Language,
			ProficiencyLevel: entry.Level,
			Category:         record.Category,
			Prompt:           record.Prompt,
			Options:          mustJSON(record.Options),
			CorrectAnswer:    record.CorrectAnswer,
			Explanation:      record.Explanation,
			Difficulty:       record.Difficulty,
		}
		if record.AudioScript != "" {
			audioScript := record.AudioScript
			question.AudioScript = &audioScript
		}
		if item.materialID != nil {
			setQuestionMaterial(
				question,
				*item.materialID,
			)
		}
		setQuestionKey(
			question,
			record.QuestionKey,
		)
		questions = append(
			questions,
			question,
		)
	}
	return questions, nil
}

func setQuestionMaterial(
	question *model.Question,
	materialID int,
) {
	id := materialID
	question.MaterialID = &id
}

func setQuestionKey(
	question *model.Question,
	questionKey string,
) {
	question.QuestionKey = &questionKey
}

func mustJSON(values []string) json.RawMessage {
	b, err := json.Marshal(values)
	if err != nil {
		panic(fmt.Sprintf(
			"marshal options: %v",
			err,
		))
	}
	return b
}
