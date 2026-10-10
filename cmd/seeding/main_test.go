package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

const japanese = "ja"

// The data/<language>/<level>.json files are the source of truth, so
// these checks catch a malformed or regressed file at test time instead of at
// seed time. They inspect the loaded records, i.e. exactly what the seeder
// consumes.

// TestRecordIntegrity checks every level file (ADR-066). Keys are the
// upsert identity, so they must be unique across levels. Only listening
// questions may sit outside a material.
func TestRecordIntegrity(t *testing.T) {
	t.Parallel()

	materials, looseQuestions, err := loadRecords(japanese)
	if err != nil {
		t.Fatalf(
			"loadRecords: %v",
			err,
		)
	}
	if len(materials) == 0 {
		t.Fatal("no material records loaded")
	}
	materialKeys := make(map[string]bool)
	questionKeys := make(map[string]bool)
	checkQuestionKey := func(question model.Question) {
		if questionKeys[*question.QuestionKey] {
			t.Fatalf(
				"duplicate question key %q",
				*question.QuestionKey,
			)
		}
		questionKeys[*question.QuestionKey] = true
	}
	for _, material := range materials {
		// Language and level are stamped from the file path; ID,
		// content_id, and created_at belong to the database.
		if !strings.HasPrefix(
			material.MaterialKey,
			japanese+":",
		) || material.Category == "" || material.Title == "" || !validDifficulty(material.Difficulty) ||
			material.Language != japanese ||
			material.ProficiencyLevel == "" ||
			material.ID != 0 ||
			material.ContentID != nil ||
			!material.CreatedAt.IsZero() {
			t.Fatalf(
				"invalid material record %q",
				material.MaterialKey,
			)
		}
		assertMaterialPayload(
			t,
			*material,
		)
		if materialKeys[material.MaterialKey] {
			t.Fatalf(
				"duplicate material key %q",
				material.MaterialKey,
			)
		}
		materialKeys[material.MaterialKey] = true
		for _, question := range material.Questions {
			assertQuestionRecord(
				t,
				question,
			)
			if question.Category == model.CategoryListening {
				t.Fatalf(
					"listening question %q must not belong to material %q",
					*question.QuestionKey,
					material.MaterialKey,
				)
			}
			checkQuestionKey(question)
		}
	}
	for _, question := range looseQuestions {
		assertQuestionRecord(
			t,
			*question,
		)
		if question.Category != model.CategoryListening {
			t.Fatalf(
				"material-less question %q must be listening, got %q",
				*question.QuestionKey,
				question.Category,
			)
		}
		checkQuestionKey(*question)
	}
}

// validDifficulty mirrors the materials/questions CHECK constraint in
// migrations/001_init.sql, so a bad value fails here instead of mid-seed.
func validDifficulty(difficulty int) bool {
	return difficulty >= 1 && difficulty <= 10
}

// requiredPayloadKeys lists, per category, the payload keys the bot's study
// cards read. Each must hold a non-empty string or a non-empty array. A new
// category must add its row here.
var requiredPayloadKeys = map[model.MaterialCategory][]string{
	model.MaterialCategoryKana:       {"kana", "romaji", "script"},
	model.MaterialCategoryVocabulary: {"kana", "kanji", "meaning_ko", "part_of_speech"},
	model.MaterialCategoryGrammar: {
		"pattern",
		"meaning_ko",
		"explanation_ko",
		"example",
		"example_reading",
		"translation_ko",
	},
	model.MaterialCategoryReading: {"passage", "reading", "key_vocabulary"},
}

func assertMaterialPayload(
	t *testing.T,
	material materialRecord,
) {
	t.Helper()
	keys, ok := requiredPayloadKeys[material.Category]
	if !ok {
		t.Fatalf(
			"material %q: no payload rules for category %q",
			material.MaterialKey,
			material.Category,
		)
	}
	var payload map[string]any
	if err := json.Unmarshal(
		material.Payload,
		&payload,
	); err != nil {
		t.Fatalf(
			"material %q payload: %v",
			material.MaterialKey,
			err,
		)
	}
	for _, key := range keys {
		var present bool
		switch value := payload[key].(type) {
		case string:
			present = value != ""
		case []any:
			present = len(value) > 0
		}
		if !present {
			t.Fatalf(
				"material %q payload has no %q: %s",
				material.MaterialKey,
				key,
				material.Payload,
			)
		}
	}
}

func assertQuestionRecord(
	t *testing.T,
	question model.Question,
) {
	t.Helper()
	// Language and level are stamped from the file path; the remaining
	// pointer fields and ID belong to the database or the seeder.
	if question.QuestionKey == nil || !strings.HasPrefix(
		*question.QuestionKey,
		japanese+":",
	) || question.Skill == nil || *question.Skill == "" || question.Type == "" || question.Category == "" ||
		question.Prompt == "" ||
		question.CorrectAnswer == "" ||
		!validDifficulty(question.Difficulty) ||
		question.Language != japanese ||
		question.ProficiencyLevel == "" ||
		question.ID != 0 ||
		question.ContentID != nil ||
		question.MaterialID != nil ||
		question.AudioPath != nil ||
		question.AudioFileID != nil ||
		!question.CreatedAt.IsZero() {
		t.Fatalf(
			"invalid question record: %+v",
			question,
		)
	}
	var options []string
	if err := json.Unmarshal(
		question.Options,
		&options,
	); err != nil {
		t.Fatalf(
			"question %q options: %v",
			*question.QuestionKey,
			err,
		)
	}
	switch question.Type {
	case model.QuestionMultipleChoice, model.QuestionReadingComp, model.QuestionListening:
		distinct := make(map[string]bool)
		for _, option := range options {
			distinct[option] = true
		}
		if len(options) != 4 || len(distinct) != 4 || !distinct[question.CorrectAnswer] {
			t.Fatalf(
				"question %q needs 4 distinct options including answer %q, got %v",
				*question.QuestionKey,
				question.CorrectAnswer,
				options,
			)
		}
	case model.QuestionWordOrder:
		if len(options) < 2 || strings.Join(
			options,
			"",
		) != question.CorrectAnswer {
			t.Fatalf(
				"word-order question %q chunks do not join to the answer",
				*question.QuestionKey,
			)
		}
	}
	if question.Category == model.CategoryListening &&
		(question.AudioScript == nil || *question.AudioScript == "") {
		t.Fatalf(
			"listening question %q has no audio script",
			*question.QuestionKey,
		)
	}
}

func questionKey(key string) *string {
	return &key
}

func TestCollectQuestionsLinksNestedQuestions(t *testing.T) {
	t.Parallel()

	vocab := model.Question{
		QuestionKey:   questionKey("ja:question:n4:sample_vocab"),
		Skill:         model.SkillPtr(model.SkillVocabKanjiReading),
		Type:          model.QuestionFillBlank,
		Category:      model.CategoryVocabulary,
		Prompt:        "「経験」의 읽기를 쓰세요.",
		Options:       []byte(`[]`),
		CorrectAnswer: "けいけん",
		Difficulty:    2,
	}
	listening := model.Question{
		QuestionKey:   questionKey("ja:listening:n4:sample_listening"),
		Skill:         model.SkillPtr(model.SkillListeningTask),
		Type:          model.QuestionListening,
		Category:      model.CategoryListening,
		Prompt:        "몇 시에 갑니까?",
		CorrectAnswer: "9시",
		Difficulty:    1,
	}
	materials := []*materialRecord{
		{Material: model.Material{ID: 42, MaterialKey: "ja:vocab:n4_word_0001"}, Questions: []model.Question{vocab}},
		{Material: model.Material{ID: 43, MaterialKey: "ja:vocab:n4_word_0002"}},
	}
	questions := collectQuestions(
		materials,
		[]*model.Question{&listening},
	)
	if len(questions) != 2 {
		t.Fatalf(
			"question count = %d, want 2",
			len(questions),
		)
	}
	if nested := questions[0]; *nested.QuestionKey != *vocab.QuestionKey || nested.MaterialID == nil ||
		*nested.MaterialID != 42 {
		t.Fatalf(
			"nested question not linked to its material: %+v",
			nested,
		)
	}
	if standalone := questions[1]; *standalone.QuestionKey != *listening.QuestionKey ||
		standalone.MaterialID != nil {
		t.Fatalf(
			"material-less question linked to a material: %+v",
			standalone,
		)
	}
}

func TestParseLanguageArg(t *testing.T) {
	t.Parallel()

	languages := []string{japanese}
	language, err := parseLanguageArg(
		[]string{japanese},
		languages,
	)
	if err != nil || language != japanese {
		t.Fatalf(
			"parseLanguageArg(%q) = %q, %v; want %q, nil",
			japanese,
			language,
			err,
			japanese,
		)
	}
	// Missing, extra, unknown and non-exact arguments all fail before any
	// loading.
	for _, args := range [][]string{
		nil,
		{japanese, "n5"},
		{""},
		{"el"},
		{"JA"},
		{" ja"},
	} {
		if _, err := parseLanguageArg(
			args,
			languages,
		); err == nil {
			t.Fatalf(
				"parseLanguageArg(%q) error = nil, want error",
				args,
			)
		}
	}
}
