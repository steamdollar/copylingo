package catalog

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

func levelCatalogForTest(
	t *testing.T,
	level string,
) LevelCatalog {
	t.Helper()
	catalog, ok := LevelCatalogFor(
		Japanese,
		level,
	)
	if !ok {
		t.Fatalf(
			"catalog for level %q not found",
			level,
		)
	}
	return catalog
}

func TestLevelCatalogRegistry(t *testing.T) {
	t.Parallel()

	catalogs := LevelCatalogsFor(Japanese)
	if len(catalogs) < 2 {
		t.Fatalf(
			"catalog count = %d, want at least 2",
			len(catalogs),
		)
	}
	if catalogs[0].Level != DefaultProficiencyLevel(Japanese) {
		t.Fatalf(
			"default level = %q, first catalog = %q",
			DefaultProficiencyLevel(Japanese),
			catalogs[0].Level,
		)
	}
	for _, catalog := range catalogs {
		if catalog.Language != Japanese {
			t.Fatalf(
				"catalog %q language = %q, want %q",
				catalog.Level,
				catalog.Language,
				Japanese,
			)
		}
		got, ok := LevelCatalogFor(
			" JA ",
			"  "+strings.ToLower(catalog.Level)+"  ",
		)
		if !ok || got.Level != catalog.Level {
			t.Fatalf(
				"normalized lookup for %q = (%q, %v)",
				catalog.Level,
				got.Level,
				ok,
			)
		}
	}
	if _, ok := LevelCatalogFor(
		Japanese,
		"N0",
	); ok {
		t.Fatal("unknown level unexpectedly resolved")
	}
	// A language without a data/<code>/ registry entry must not borrow
	// another language's catalogs.
	if got := LevelCatalogsFor("el"); len(got) != 0 {
		t.Fatalf(
			"unregistered language catalogs = %d, want 0",
			len(got),
		)
	}
	if _, ok := LevelCatalogFor(
		"el",
		catalogs[0].Level,
	); ok {
		t.Fatal("level resolved for an unregistered language")
	}
}

func contains(
	values []string,
	want string,
) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// The data/<language>/<level>.json files are the source of truth, so
// these checks catch a malformed or regressed file at test time instead of at
// seed time. They inspect the loaded catalogs, i.e. exactly what the seeder
// consumes.

// TestRecordCatalogIntegrity checks every registered level (ADR-066). Keys are
// the upsert identity, so they must be unique across levels. Only listening
// questions may sit outside a material.
func TestRecordCatalogIntegrity(t *testing.T) {
	t.Parallel()

	materialKeys := make(map[string]bool)
	questionKeys := make(map[string]bool)
	checkQuestionKey := func(record QuestionRecord) {
		if questionKeys[record.QuestionKey] {
			t.Fatalf(
				"duplicate question key %q",
				record.QuestionKey,
			)
		}
		questionKeys[record.QuestionKey] = true
	}
	for _, catalog := range LevelCatalogsFor(Japanese) {
		if len(catalog.Materials) == 0 {
			t.Fatalf(
				"level %s has no material records",
				catalog.Level,
			)
		}
		for _, material := range catalog.Materials {
			if !strings.HasPrefix(
				material.MaterialKey,
				catalog.Language+":",
			) || material.Category == "" || material.Title == "" || !validDifficulty(material.Difficulty) {
				t.Fatalf(
					"invalid %s material record %q",
					catalog.Level,
					material.MaterialKey,
				)
			}
			assertMaterialPayload(
				t,
				catalog.Level,
				material,
			)
			if materialKeys[material.MaterialKey] {
				t.Fatalf(
					"duplicate material key %q",
					material.MaterialKey,
				)
			}
			materialKeys[material.MaterialKey] = true
			for _, record := range material.Questions {
				assertQuestionRecord(
					t,
					catalog,
					record,
				)
				if record.Category == model.CategoryListening {
					t.Fatalf(
						"listening question %q must not belong to material %q",
						record.QuestionKey,
						material.MaterialKey,
					)
				}
				checkQuestionKey(record)
			}
		}
		for _, record := range catalog.Questions {
			assertQuestionRecord(
				t,
				catalog,
				record,
			)
			if record.Category != model.CategoryListening {
				t.Fatalf(
					"material-less question %q must be listening, got %q",
					record.QuestionKey,
					record.Category,
				)
			}
			checkQuestionKey(record)
		}
	}
}

// validDifficulty mirrors the materials/questions CHECK constraint in
// migrations/001_init.sql, so a bad value fails here instead of mid-seed.
func validDifficulty(difficulty int) bool {
	return difficulty >= 1 && difficulty <= 10
}

// assertMaterialPayload checks the payload fields the bot's study cards read
// per category. A new category must add its rules here.
func assertMaterialPayload(
	t *testing.T,
	level string,
	material MaterialRecord,
) {
	t.Helper()
	var complete bool
	switch material.Category {
	case model.MaterialCategoryKana:
		var payload KanaMaterialPayload
		complete = json.Unmarshal(
			material.Payload,
			&payload,
		) == nil &&
			payload.Kana != "" && payload.Romaji != "" && payload.Script != ""
	case model.MaterialCategoryVocabulary:
		var payload VocabularyMaterialPayload
		complete = json.Unmarshal(
			material.Payload,
			&payload,
		) == nil &&
			payload.Kana != "" && payload.Kanji != "" && payload.MeaningKo != "" && payload.PartOfSpeech != ""
	case model.MaterialCategoryGrammar:
		var payload GrammarMaterialPayload
		complete = json.Unmarshal(
			material.Payload,
			&payload,
		) == nil &&
			payload.Pattern != "" && payload.MeaningKo != "" && payload.ExplanationKo != "" &&
			payload.Example != "" && payload.ExampleReading != "" && payload.TranslationKo != ""
	case model.MaterialCategoryReading:
		var payload ReadingMaterialPayload
		complete = json.Unmarshal(
			material.Payload,
			&payload,
		) == nil &&
			payload.Passage != "" && payload.Reading != "" && len(payload.KeyVocabulary) > 0
	default:
		t.Fatalf(
			"%s material %q: no payload rules for category %q",
			level,
			material.MaterialKey,
			material.Category,
		)
	}
	if !complete {
		t.Fatalf(
			"%s material %q has an incomplete %s payload: %s",
			level,
			material.MaterialKey,
			material.Category,
			material.Payload,
		)
	}
}

func assertQuestionRecord(
	t *testing.T,
	catalog LevelCatalog,
	record QuestionRecord,
) {
	t.Helper()
	if !strings.HasPrefix(
		record.QuestionKey,
		catalog.Language+":",
	) || record.ItemType == "" || record.Type == "" || record.Category == "" || record.Prompt == "" ||
		record.CorrectAnswer == "" ||
		!validDifficulty(record.Difficulty) {
		t.Fatalf(
			"invalid %s question record: %+v",
			catalog.Level,
			record,
		)
	}
	switch record.Type {
	case model.QuestionMultipleChoice, model.QuestionReadingComp, model.QuestionListening:
		distinct := make(map[string]bool)
		for _, option := range record.Options {
			distinct[option] = true
		}
		if len(record.Options) != 4 || len(distinct) != 4 || !distinct[record.CorrectAnswer] {
			t.Fatalf(
				"question %q needs 4 distinct options including answer %q, got %v",
				record.QuestionKey,
				record.CorrectAnswer,
				record.Options,
			)
		}
	case model.QuestionWordOrder:
		if len(record.Options) < 2 || strings.Join(
			record.Options,
			"",
		) != record.CorrectAnswer {
			t.Fatalf(
				"word-order question %q chunks do not join to the answer",
				record.QuestionKey,
			)
		}
	}
	if record.Category == model.CategoryListening && record.AudioScript == "" {
		t.Fatalf(
			"listening question %q has no audio script",
			record.QuestionKey,
		)
	}
}

// TestRecordLevelsCoverItemTypes pins each level's item types, so a dropped
// record group is caught. N4 follows the JLPT taxonomy; N5 keeps the app's
// beginner drills from before ADR-067 alongside JLPT-style items.
func TestRecordLevelsCoverItemTypes(t *testing.T) {
	t.Parallel()

	wantByLevel := map[string][]model.Skill{
		"N5": {
			model.SkillKanaReading,
			model.SkillKanaRecall,
			model.SkillKanaHandwriting,
			model.SkillVocabMeaning,
			model.SkillVocabRecall,
			model.SkillVocabHandwriting,
			model.SkillVocabKanjiRecall,
			model.SkillVocabContext,
			model.SkillGrammarForm,
			model.SkillSentenceComposition,
			model.SkillReadingShort,
			model.SkillListeningTask,
			model.SkillListeningKeyPoint,
			model.SkillListeningOutline,
		},
		"N4": {
			model.SkillVocabKanjiReading,
			model.SkillVocabOrthography,
			model.SkillVocabContext,
			model.SkillVocabParaphrase,
			model.SkillVocabUsage,
			model.SkillGrammarForm,
			model.SkillSentenceComposition,
			model.SkillGrammarText,
			model.SkillReadingShort,
			model.SkillReadingMedium,
			model.SkillReadingInformation,
			model.SkillListeningTask,
			model.SkillListeningKeyPoint,
			model.SkillListeningVerbal,
			model.SkillListeningQuickResponse,
		},
	}
	for level, skills := range wantByLevel {
		want := make(map[model.Skill]bool)
		for _, skill := range skills {
			want[skill] = true
		}
		catalog := levelCatalogForTest(
			t,
			level,
		)
		got := make(map[model.Skill]bool)
		for _, material := range catalog.Materials {
			for _, record := range material.Questions {
				got[record.ItemType] = true
			}
		}
		for _, record := range catalog.Questions {
			got[record.ItemType] = true
		}
		if !reflect.DeepEqual(
			got,
			want,
		) {
			t.Fatalf(
				"%s item types = %v, want %v",
				level,
				got,
				want,
			)
		}
	}
}
