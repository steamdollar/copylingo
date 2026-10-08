package main

import (
	"context"
	"testing"

	"github.com/lsj/copylingo/cmd/seeding/catalog"
	"github.com/lsj/copylingo/internal/model"
)

func TestBuildRecordQuestionsMapsRecords(t *testing.T) {
	t.Parallel()

	vocab := catalog.QuestionRecord{
		QuestionKey:   "ja:question:n4:sample_vocab",
		ItemType:      model.SkillVocabKanjiReading,
		Type:          model.QuestionFillBlank,
		Category:      model.CategoryVocabulary,
		Prompt:        "「経験」의 읽기를 쓰세요.",
		Options:       []string{},
		CorrectAnswer: "けいけん",
		Explanation:   "経験은 けいけん으로 읽습니다.",
		Difficulty:    2,
	}
	listening := catalog.QuestionRecord{
		QuestionKey:   "ja:listening:n4:sample_listening",
		ItemType:      model.SkillListeningTask,
		Type:          model.QuestionListening,
		Category:      model.CategoryListening,
		Prompt:        "몇 시에 갑니까?",
		Options:       []string{"9시", "10시"},
		CorrectAnswer: "9시",
		AudioScript:   "九時に行きます。",
		Difficulty:    1,
	}
	entry := levelCatalog{
		Language: catalog.Japanese,
		Level:    "N4",
		Materials: []catalog.MaterialRecord{
			{MaterialKey: "ja:vocab:n4_word_0001", Questions: []catalog.QuestionRecord{vocab}},
			{MaterialKey: "ja:vocab:n4_word_0002"},
		},
		Questions: []catalog.QuestionRecord{listening},
	}
	questions, err := buildRecordQuestions(
		entry,
		map[string]int{"ja:vocab:n4_word_0001": 42},
	)
	if err != nil {
		t.Fatalf(
			"buildRecordQuestions: %v",
			err,
		)
	}
	if len(questions) != 2 {
		t.Fatalf(
			"question count = %d, want 2",
			len(questions),
		)
	}
	nested := questions[0]
	if nested.QuestionKey == nil || *nested.QuestionKey != vocab.QuestionKey ||
		nested.Language != catalog.Japanese ||
		nested.ProficiencyLevel != "N4" ||
		nested.MaterialID == nil ||
		*nested.MaterialID != 42 ||
		nested.Skill == nil ||
		*nested.Skill != model.SkillVocabKanjiReading ||
		string(nested.Options) != "[]" ||
		nested.AudioScript != nil {
		t.Fatalf(
			"nested question mapped incorrectly: %+v",
			nested,
		)
	}
	standalone := questions[1]
	if standalone.MaterialID != nil || standalone.AudioScript == nil ||
		*standalone.AudioScript != listening.AudioScript {
		t.Fatalf(
			"material-less question mapped incorrectly: %+v",
			standalone,
		)
	}
}

func TestBuildRecordQuestionsRejectsInvalidRecords(t *testing.T) {
	t.Parallel()

	const materialKey = "ja:grammar:n4_grammar_0001"
	valid := catalog.QuestionRecord{
		QuestionKey:   "ja:question:n4:valid",
		ItemType:      model.SkillGrammarForm,
		Type:          model.QuestionMultipleChoice,
		Category:      model.CategoryGrammar,
		Prompt:        "빈칸에 들어갈 표현을 고르세요.",
		Options:       []string{"そうです", "ようです"},
		CorrectAnswer: "そうです",
		Difficulty:    2,
	}
	nestedUnder := func(
		key string,
		records ...catalog.QuestionRecord,
	) levelCatalog {
		return levelCatalog{
			Language:  catalog.Japanese,
			Level:     "N4",
			Materials: []catalog.MaterialRecord{{MaterialKey: key, Questions: records}},
		}
	}
	tests := []struct {
		name  string
		entry func() levelCatalog
	}{
		{
			name: "missing prompt",
			entry: func() levelCatalog {
				record := valid
				record.Prompt = ""
				return nestedUnder(
					materialKey,
					record,
				)
			},
		},
		{
			name: "zero difficulty",
			entry: func() levelCatalog {
				record := valid
				record.Difficulty = 0
				return nestedUnder(
					materialKey,
					record,
				)
			},
		},
		{
			name: "duplicate key across nested and top-level",
			entry: func() levelCatalog {
				entry := nestedUnder(
					materialKey,
					valid,
				)
				entry.Questions = []catalog.QuestionRecord{valid}
				return entry
			},
		},
		{
			name: "listening nested under material",
			entry: func() levelCatalog {
				record := valid
				record.Category = model.CategoryListening
				return nestedUnder(
					materialKey,
					record,
				)
			},
		},
		{
			name: "unresolved material",
			entry: func() levelCatalog {
				return nestedUnder(
					"ja:grammar:missing",
					valid,
				)
			},
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				t.Parallel()

				if _, err := buildRecordQuestions(
					tt.entry(),
					map[string]int{materialKey: 1},
				); err == nil {
					t.Fatal("buildRecordQuestions accepted an invalid record")
				}
			},
		)
	}
}

func TestLoadRecordMaterialIDsRequestsOnlyMaterialsWithQuestions(t *testing.T) {
	t.Parallel()

	store := &recordingMaterialStore{
		materials: []model.Material{
			{ID: 7, MaterialKey: "ja:vocab:n4_word_0001"},
		},
	}
	catalogs := []levelCatalog{
		{
			Materials: []catalog.MaterialRecord{
				{
					MaterialKey: "ja:vocab:n4_word_0001",
					Questions:   []catalog.QuestionRecord{{QuestionKey: "ja:question:n4:a"}},
				},
				{MaterialKey: "ja:vocab:n4_word_0002"},
			},
		},
	}
	ids, err := loadRecordMaterialIDs(
		context.Background(),
		store,
		catalogs,
	)
	if err != nil {
		t.Fatalf(
			"loadRecordMaterialIDs: %v",
			err,
		)
	}
	if len(store.requestedKeys) != 1 || store.requestedKeys[0] != "ja:vocab:n4_word_0001" {
		t.Fatalf(
			"requested keys = %v, want only the material with questions",
			store.requestedKeys,
		)
	}
	if ids["ja:vocab:n4_word_0001"] != 7 {
		t.Fatalf(
			"resolved ids = %v",
			ids,
		)
	}
}

type recordingMaterialStore struct {
	materials     []model.Material
	requestedKeys []string
}

func (s *recordingMaterialStore) GetByMaterialKeys(
	ctx context.Context,
	keys []string,
) ([]model.Material, error) {
	s.requestedKeys = append(
		s.requestedKeys,
		keys...,
	)
	return s.materials, nil
}
