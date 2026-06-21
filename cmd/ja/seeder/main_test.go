package main

import (
	"context"
	"math/rand"
	"strings"
	"testing"

	ja "github.com/lsj/copylingo/cmd/ja"
	"github.com/lsj/copylingo/internal/model"
)

func TestKanaScriptLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		kana string
		want string
	}{
		{name: "hiragana basic", kana: "あ", want: "히라가나"},
		{name: "hiragana yoon", kana: "きゃ", want: "히라가나"},
		{name: "katakana basic", kana: "ア", want: "가타카나"},
		{name: "katakana yoon", kana: "キャ", want: "가타카나"},
		{name: "unknown", kana: "a", want: "가나"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := kanaScriptLabel(tt.kana); got != tt.want {
				t.Fatalf("kanaScriptLabel(%q) = %q, want %q", tt.kana, got, tt.want)
			}
		})
	}
}

func TestBuildQuestionType2PromptIncludesScriptLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		answer string
		want   string
	}{
		{name: "hiragana answer", answer: "あ", want: "히라가나 문자"},
		{name: "katakana answer", answer: "ア", want: "가타카나 문자"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			q := buildQuestion("a", tt.answer, []string{"い", "う", "え", "お"}, false)
			if !strings.Contains(q.Prompt, tt.want) {
				t.Fatalf("prompt %q does not contain %q", q.Prompt, tt.want)
			}
			if !strings.Contains(q.Explanation, tt.want) {
				t.Fatalf("explanation %q does not contain %q", q.Explanation, tt.want)
			}
		})
	}
}

func TestBuildQuestionSetsSkill(t *testing.T) {
	t.Parallel()

	toRomaji := buildQuestion("あ", "a", []string{"i", "u", "e", "o"}, true)
	if toRomaji.Skill == nil || *toRomaji.Skill != model.SkillKanaReading {
		t.Fatalf("toRomaji skill = %v, want %q", toRomaji.Skill, model.SkillKanaReading)
	}

	toKana := buildQuestion("a", "あ", []string{"い", "う", "え", "お"}, false)
	if toKana.Skill == nil || *toKana.Skill != model.SkillKanaRecall {
		t.Fatalf("toKana skill = %v, want %q", toKana.Skill, model.SkillKanaRecall)
	}

	handwriting := buildHandwritingQuestion("a", "あ")
	if handwriting.Skill == nil || *handwriting.Skill != model.SkillKanaHandwriting {
		t.Fatalf("handwriting skill = %v, want %q", handwriting.Skill, model.SkillKanaHandwriting)
	}
}

func TestKanaDisambiguationHint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		kana string
		want string
	}{
		{name: "hiragana ji sa row", kana: "じ", want: "さ행에 탁점"},
		{name: "hiragana zu sa row", kana: "ず", want: "さ행에 탁점"},
		{name: "hiragana ji ta row", kana: "ぢ", want: "た행에 탁점"},
		{name: "hiragana zu ta row", kana: "づ", want: "た행에 탁점"},
		{name: "katakana ji sa row", kana: "ジ", want: "サ행에 탁점"},
		{name: "katakana zu sa row", kana: "ズ", want: "サ행에 탁점"},
		{name: "katakana ji ta row", kana: "ヂ", want: "タ행에 탁점"},
		{name: "katakana zu ta row", kana: "ヅ", want: "タ행에 탁점"},
		{name: "unambiguous kana", kana: "が", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := kanaDisambiguationHint(tt.kana); got != tt.want {
				t.Fatalf("kanaDisambiguationHint(%q) = %q, want %q", tt.kana, got, tt.want)
			}
		})
	}
}

func TestAmbiguousReverseKanaQuestionsIncludeHint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		kana string
		want string
	}{
		{name: "hiragana ji sa row", kana: "じ", want: "さ행에 탁점"},
		{name: "hiragana zu sa row", kana: "ず", want: "さ행에 탁점"},
		{name: "hiragana ji ta row", kana: "ぢ", want: "た행에 탁점"},
		{name: "hiragana zu ta row", kana: "づ", want: "た행에 탁점"},
		{name: "katakana ji sa row", kana: "ジ", want: "サ행에 탁점"},
		{name: "katakana zu sa row", kana: "ズ", want: "サ행에 탁점"},
		{name: "katakana ji ta row", kana: "ヂ", want: "タ행에 탁점"},
		{name: "katakana zu ta row", kana: "ヅ", want: "タ행에 탁점"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			romaji := ja.KanaMap[tt.kana]
			type2 := buildQuestion(romaji, tt.kana, []string{"あ", "い", "う", "え", "お"}, false)
			if !strings.Contains(type2.Prompt, tt.want) {
				t.Fatalf("Type 2 prompt %q does not contain %q", type2.Prompt, tt.want)
			}

			handwriting := buildHandwritingQuestion(romaji, tt.kana)
			if !strings.Contains(handwriting.Prompt, tt.want) {
				t.Fatalf("handwriting prompt %q does not contain %q", handwriting.Prompt, tt.want)
			}
		})
	}
}

func TestUnambiguousReverseKanaQuestionsDoNotIncludeHint(t *testing.T) {
	t.Parallel()

	type2 := buildQuestion("ga", "が", []string{"あ", "い", "う", "え", "お"}, false)
	if strings.Contains(type2.Prompt, "힌트:") {
		t.Fatalf("Type 2 prompt unexpectedly contains hint: %q", type2.Prompt)
	}

	handwriting := buildHandwritingQuestion("ga", "が")
	if strings.Contains(handwriting.Prompt, "힌트:") {
		t.Fatalf("handwriting prompt unexpectedly contains hint: %q", handwriting.Prompt)
	}
}

func TestShouldSeedHandwritingQuestion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		kana string
		want bool
	}{
		{name: "katakana yu excluded", kana: "ユ", want: false},
		{name: "katakana wo excluded", kana: "ヲ", want: false},
		{name: "hiragana yu remains", kana: "ゆ", want: true},
		{name: "hiragana wo remains", kana: "を", want: true},
		{name: "other katakana remains", kana: "ヨ", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := shouldSeedHandwritingQuestion(tt.kana); got != tt.want {
				t.Fatalf("shouldSeedHandwritingQuestion(%q) = %t, want %t", tt.kana, got, tt.want)
			}
		})
	}
}

func TestBuildKanaQuestionsSetsStableQuestionKeys(t *testing.T) {
	t.Parallel()

	materialIDsByKana := make(map[string]int, len(ja.KanaMap))
	for kana := range ja.KanaMap {
		materialIDsByKana[kana] = len(materialIDsByKana) + 1
	}

	questions := buildKanaQuestions(materialIDsByKana)
	if len(questions) != 622 {
		t.Fatalf("len(questions) = %d, want 622", len(questions))
	}

	seen := make(map[string]bool, len(questions))
	for _, question := range questions {
		if question.QuestionKey == nil || *question.QuestionKey == "" {
			t.Fatalf("missing question_key for question: %+v", question)
		}
		if seen[*question.QuestionKey] {
			t.Fatalf("duplicate question_key %q", *question.QuestionKey)
		}
		seen[*question.QuestionKey] = true
		if question.MaterialID == nil {
			t.Fatalf("missing material_id for question_key %q", *question.QuestionKey)
		}
	}

	for _, key := range []string{
		"ja:kana:u3042:reading",
		"ja:kana:u3042:recall",
		"ja:kana:u3042:handwriting",
		"ja:kana:u30e6:reading",
		"ja:kana:u30e6:recall",
	} {
		if !seen[key] {
			t.Fatalf("question_key %q not found", key)
		}
	}
	if seen["ja:kana:u30e6:handwriting"] {
		t.Fatal("question_key ja:kana:u30e6:handwriting found, want excluded handwriting")
	}
}

func TestLoadKanaMaterialIDs(t *testing.T) {
	t.Parallel()

	store := fakeKanaMaterialStore{
		materials: []model.Material{
			{ID: 1, MaterialKey: "ja:kana:u3042"},
			{ID: 2, MaterialKey: "ja:kana:u304d_u3083"},
		},
	}

	got, err := loadKanaMaterialIDs(context.Background(), store, map[string]string{
		"あ":  "a",
		"きゃ": "kya",
	})
	if err != nil {
		t.Fatalf("loadKanaMaterialIDs: %v", err)
	}
	if got["あ"] != 1 || got["きゃ"] != 2 {
		t.Fatalf("material ids = %#v, want あ=1 and きゃ=2", got)
	}
}

func TestLoadKanaMaterialIDsMissing(t *testing.T) {
	t.Parallel()

	_, err := loadKanaMaterialIDs(context.Background(), fakeKanaMaterialStore{}, map[string]string{"あ": "a"})
	if err == nil {
		t.Fatal("loadKanaMaterialIDs error = nil, want missing material error")
	}
	if !strings.Contains(err.Error(), "ja:kana:u3042") {
		t.Fatalf("error = %q, want missing material key", err)
	}
}

type fakeKanaMaterialStore struct {
	materials []model.Material
	err       error
}

func (s fakeKanaMaterialStore) GetByMaterialKeys(
	ctx context.Context,
	keys []string,
) ([]model.Material, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.materials, nil
}

func TestBuildKanaToMeaningQuestion(t *testing.T) {
	t.Parallel()

	word := vocabWord{ID: "n5_word_test", Kana: "みず", Kanji: "水", MeaningKo: "물", PartOfSpeech: "noun"}
	q := buildKanaToMeaningQuestion(rand.New(rand.NewSource(1)), word, n5Words)

	if q.Type != model.QuestionMultipleChoice {
		t.Fatalf("type = %q, want %q", q.Type, model.QuestionMultipleChoice)
	}
	if q.Skill == nil || *q.Skill != model.SkillVocabMeaning {
		t.Fatalf("skill = %v, want %q", q.Skill, model.SkillVocabMeaning)
	}
	if q.Category != model.CategoryVocabulary {
		t.Fatalf("category = %q, want %q", q.Category, model.CategoryVocabulary)
	}
	if q.Language != vocabLanguage {
		t.Fatalf("language = %q, want %q", q.Language, vocabLanguage)
	}
	if q.ProficiencyLevel != vocabProficiencyLevel {
		t.Fatalf("level = %q, want %q", q.ProficiencyLevel, vocabProficiencyLevel)
	}
	if q.Difficulty != vocabDifficulty {
		t.Fatalf("difficulty = %d, want %d", q.Difficulty, vocabDifficulty)
	}

	options, err := q.GetOptions()
	if err != nil {
		t.Fatalf("GetOptions: %v", err)
	}
	if len(options) != 4 {
		t.Fatalf("len(options) = %d, want 4: %v", len(options), options)
	}

	seen := make(map[string]bool, len(options))
	hasAnswer := false
	for _, opt := range options {
		if seen[opt] {
			t.Fatalf("duplicate option %q in %v", opt, options)
		}
		seen[opt] = true
		if opt == word.MeaningKo {
			hasAnswer = true
		}
	}
	if !hasAnswer {
		t.Fatalf("options %v do not contain answer %q", options, word.MeaningKo)
	}
	assertContainsAll(t, q.Explanation, word.Kana, word.Kanji, word.MeaningKo)
}

func TestBuildMeaningToKanaQuestion(t *testing.T) {
	t.Parallel()

	word := vocabWord{ID: "n5_word_test", Kana: "みず", Kanji: "水", MeaningKo: "물", PartOfSpeech: "noun"}
	q := buildMeaningToKanaQuestion(word)

	if q.Type != model.QuestionFillBlank {
		t.Fatalf("type = %q, want %q", q.Type, model.QuestionFillBlank)
	}
	if q.Skill == nil || *q.Skill != model.SkillVocabRecall {
		t.Fatalf("skill = %v, want %q", q.Skill, model.SkillVocabRecall)
	}
	if q.CorrectAnswer != word.Kana {
		t.Fatalf("correct answer = %q, want %q", q.CorrectAnswer, word.Kana)
	}
	options, err := q.GetOptions()
	if err != nil {
		t.Fatalf("GetOptions: %v", err)
	}
	if len(options) != 0 {
		t.Fatalf("len(options) = %d, want 0: %v", len(options), options)
	}
	if !strings.Contains(q.Prompt, word.MeaningKo) {
		t.Fatalf("prompt %q does not contain meaning %q", q.Prompt, word.MeaningKo)
	}
}

func TestBuildMeaningToKanaQuestionCounterUsesKanjiReadingPrompt(t *testing.T) {
	t.Parallel()

	word := vocabWord{ID: "n5_word_test", Kana: "にほん", Kanji: "二本", MeaningKo: "두 개 (길고 가는 것)", PartOfSpeech: "counter"}
	q := buildMeaningToKanaQuestion(word)

	assertContainsAll(t, q.Prompt, "二本", "두 개 (길고 가는 것)", "히라가나 읽기")
	if strings.Contains(q.Prompt, "にほん") {
		t.Fatalf("counter prompt %q should not reveal answer kana", q.Prompt)
	}
	if strings.Contains(q.Prompt, "뜻 <b>'두 개") {
		t.Fatalf("counter prompt %q should not ask by meaning only", q.Prompt)
	}
}

func TestBuildMeaningToKanaHandwritingQuestion(t *testing.T) {
	t.Parallel()

	word := vocabWord{ID: "n5_word_test", Kana: "がっこう", Kanji: "学校", MeaningKo: "학교", PartOfSpeech: "noun"}
	q := buildMeaningToKanaHandwritingQuestion(word)

	if q.Type != model.QuestionKanaHandwriting {
		t.Fatalf("type = %q, want %q", q.Type, model.QuestionKanaHandwriting)
	}
	if q.Skill == nil || *q.Skill != model.SkillVocabHandwriting {
		t.Fatalf("skill = %v, want %q", q.Skill, model.SkillVocabHandwriting)
	}
	if q.Category != model.CategoryVocabulary {
		t.Fatalf("category = %q, want %q", q.Category, model.CategoryVocabulary)
	}
	if q.CorrectAnswer != word.Kana {
		t.Fatalf("correct answer = %q, want %q", q.CorrectAnswer, word.Kana)
	}
	if !strings.Contains(q.Prompt, word.MeaningKo) {
		t.Fatalf("prompt %q does not contain meaning %q", q.Prompt, word.MeaningKo)
	}
	assertContainsAll(t, q.Explanation, word.Kana, word.Kanji, word.MeaningKo)
}

func TestBuildMeaningToKanaHandwritingQuestionCounterUsesKanjiReadingPrompt(t *testing.T) {
	t.Parallel()

	word := vocabWord{
		ID:           "n5_word_test",
		Kana:         "さんまい",
		Kanji:        "三枚",
		MeaningKo:    "세 장 (얇고 평평한 것)",
		PartOfSpeech: "counter",
	}
	q := buildMeaningToKanaHandwritingQuestion(word)

	assertContainsAll(t, q.Prompt, "三枚", "세 장 (얇고 평평한 것)", "히라가나 읽기", "손글씨")
	if strings.Contains(q.Prompt, "さんまい") {
		t.Fatalf("counter prompt %q should not reveal answer kana", q.Prompt)
	}
	if strings.Contains(q.Prompt, "뜻 <b>'세 장") {
		t.Fatalf("counter prompt %q should not ask by meaning only", q.Prompt)
	}
}

func TestSeederN5WordsIntegrity(t *testing.T) {
	t.Parallel()

	if len(n5Words) != 500 {
		t.Fatalf("len(n5Words) = %d, want 500", len(n5Words))
	}

	ids := make(map[string]bool, len(n5Words))
	for _, word := range n5Words {
		if word.ID == "" {
			t.Fatalf("empty ID for word %+v", word)
		}
		if ids[word.ID] {
			t.Fatalf("duplicate ID %q", word.ID)
		}
		ids[word.ID] = true
		if word.Kana == "" {
			t.Fatalf("empty Kana for word %+v", word)
		}
		if word.Kanji == "" {
			t.Fatalf("empty Kanji for word %+v", word)
		}
		if word.MeaningKo == "" {
			t.Fatalf("empty MeaningKo for word %+v", word)
		}
	}
}

func TestBuildVocabularyQuestions(t *testing.T) {
	t.Parallel()

	materialIDsByWordID := make(map[string]int, len(n5Words))
	for idx, word := range n5Words {
		materialIDsByWordID[word.ID] = idx + 1
	}

	questions := buildVocabularyQuestions(rand.New(rand.NewSource(1)), n5Words, materialIDsByWordID)
	if len(questions) != 1500 {
		t.Fatalf("len(questions) = %d, want 1500", len(questions))
	}

	countByType := map[model.QuestionType]int{}
	seenKeys := make(map[string]bool, len(questions))
	for _, q := range questions {
		countByType[q.Type]++
		if q.Language != vocabLanguage || q.ProficiencyLevel != vocabProficiencyLevel ||
			q.Category != model.CategoryVocabulary || q.Difficulty != vocabDifficulty {
			t.Fatalf("unexpected question metadata: %+v", q)
		}
		if q.MaterialID == nil {
			t.Fatalf("material_id is nil for question: %+v", q)
		}
		if q.QuestionKey == nil || *q.QuestionKey == "" {
			t.Fatalf("question_key is nil for question: %+v", q)
		}
		if seenKeys[*q.QuestionKey] {
			t.Fatalf("duplicate question_key %q", *q.QuestionKey)
		}
		seenKeys[*q.QuestionKey] = true
	}

	if countByType[model.QuestionMultipleChoice] != 500 {
		t.Fatalf("multiple_choice count = %d, want 500", countByType[model.QuestionMultipleChoice])
	}
	if countByType[model.QuestionFillBlank] != 500 {
		t.Fatalf("fill_blank count = %d, want 500", countByType[model.QuestionFillBlank])
	}
	if countByType[model.QuestionKanaHandwriting] != 500 {
		t.Fatalf("kana_handwriting count = %d, want 500", countByType[model.QuestionKanaHandwriting])
	}
	for _, key := range []string{
		"ja:vocab:word_001:meaning",
		"ja:vocab:word_001:recall",
		"ja:vocab:word_001:handwriting",
	} {
		if !seenKeys[key] {
			t.Fatalf("question_key %q not found", key)
		}
	}
}

func TestVocabularyMaterialKey(t *testing.T) {
	t.Parallel()

	word := vocabWord{ID: "n5_word_024"}
	if got, want := vocabularyMaterialKey(word), "ja:vocab:word_024"; got != want {
		t.Fatalf("vocabularyMaterialKey = %q, want %q", got, want)
	}
}

func TestLoadVocabularyMaterialIDs(t *testing.T) {
	t.Parallel()

	words := []vocabWord{
		{ID: "n5_word_001"},
		{ID: "n5_word_024"},
	}
	store := fakeVocabularyMaterialStore{
		materials: []model.Material{
			{ID: 10, MaterialKey: "ja:vocab:word_001"},
			{ID: 24, MaterialKey: "ja:vocab:word_024"},
		},
	}

	got, err := loadVocabularyMaterialIDs(context.Background(), store, words)
	if err != nil {
		t.Fatalf("loadVocabularyMaterialIDs: %v", err)
	}

	if got["n5_word_001"] != 10 || got["n5_word_024"] != 24 {
		t.Fatalf("material ids = %#v, want word_001=10 and word_024=24", got)
	}
}

func TestLoadVocabularyMaterialIDsMissing(t *testing.T) {
	t.Parallel()

	_, err := loadVocabularyMaterialIDs(
		context.Background(),
		fakeVocabularyMaterialStore{},
		[]vocabWord{{ID: "n5_word_024"}},
	)
	if err == nil {
		t.Fatal("loadVocabularyMaterialIDs error = nil, want missing material error")
	}
	if !strings.Contains(err.Error(), "ja:vocab:word_024") {
		t.Fatalf("error = %q, want missing material key", err)
	}
}

type fakeVocabularyMaterialStore struct {
	materials []model.Material
	err       error
}

func (s fakeVocabularyMaterialStore) GetByMaterialKeys(
	ctx context.Context,
	keys []string,
) ([]model.Material, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.materials, nil
}

func assertContainsAll(t *testing.T, s string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(s, want) {
			t.Fatalf("%q does not contain %q", s, want)
		}
	}
}
