package main

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

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

func TestN5WordsIntegrity(t *testing.T) {
	t.Parallel()

	if len(n5Words) != 100 {
		t.Fatalf("len(n5Words) = %d, want 100", len(n5Words))
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

	questions := buildVocabularyQuestions(rand.New(rand.NewSource(1)), n5Words)
	if len(questions) != 300 {
		t.Fatalf("len(questions) = %d, want 300", len(questions))
	}

	countByType := map[model.QuestionType]int{}
	for _, q := range questions {
		countByType[q.Type]++
		if q.Language != vocabLanguage || q.ProficiencyLevel != vocabProficiencyLevel ||
			q.Category != model.CategoryVocabulary || q.Difficulty != vocabDifficulty {
			t.Fatalf("unexpected question metadata: %+v", q)
		}
	}

	if countByType[model.QuestionMultipleChoice] != 100 {
		t.Fatalf("multiple_choice count = %d, want 100", countByType[model.QuestionMultipleChoice])
	}
	if countByType[model.QuestionFillBlank] != 100 {
		t.Fatalf("fill_blank count = %d, want 100", countByType[model.QuestionFillBlank])
	}
	if countByType[model.QuestionKanaHandwriting] != 100 {
		t.Fatalf("kana_handwriting count = %d, want 100", countByType[model.QuestionKanaHandwriting])
	}
}

func assertContainsAll(t *testing.T, s string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(s, want) {
			t.Fatalf("%q does not contain %q", s, want)
		}
	}
}
