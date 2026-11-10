package model

import (
	"encoding/json"
	"testing"
)

func TestQuestion_GetOptions(t *testing.T) {
	t.Parallel()

	t.Run(
		"parses JSONB array",
		func(t *testing.T) {
			t.Parallel()
			question := &Question{Options: json.RawMessage(`["A","B","C","D"]`)}
			options, err := question.GetOptions()
			if err != nil {
				t.Fatalf(
					"GetOptions() error = %v",
					err,
				)
			}
			if len(options) != 4 || options[0] != "A" || options[3] != "D" {
				t.Fatalf(
					"GetOptions() = %v, want [A B C D]",
					options,
				)
			}
		},
	)

	t.Run(
		"empty array",
		func(t *testing.T) {
			t.Parallel()
			question := &Question{Options: json.RawMessage(`[]`)}
			options, err := question.GetOptions()
			if err != nil {
				t.Fatalf(
					"GetOptions() error = %v",
					err,
				)
			}
			if len(options) != 0 {
				t.Fatalf(
					"GetOptions() = %v, want empty",
					options,
				)
			}
		},
	)

	t.Run(
		"invalid JSON returns error",
		func(t *testing.T) {
			t.Parallel()
			question := &Question{Options: json.RawMessage(`{"not":"an array"}`)}
			if _, err := question.GetOptions(); err == nil {
				t.Fatal("GetOptions() error = nil, want unmarshal error")
			}
		},
	)

	t.Run(
		"nil options returns error",
		func(t *testing.T) {
			t.Parallel()
			question := &Question{}
			if _, err := question.GetOptions(); err == nil {
				t.Fatal("GetOptions() on nil options error = nil, want error")
			}
		},
	)
}

func TestQuestion_ShuffleOptions(t *testing.T) {
	t.Parallel()

	t.Run(
		"deterministic for same session and question ID",
		func(t *testing.T) {
			t.Parallel()
			raw := `["optA","optB","optC","optD"]`
			firstQuestion := &Question{ID: 42, Options: json.RawMessage(raw)}
			secondQuestion := &Question{ID: 42, Options: json.RawMessage(raw)}

			if err := firstQuestion.ShuffleOptions(101); err != nil {
				t.Fatalf(
					"ShuffleOptions() err = %v",
					err,
				)
			}
			if err := secondQuestion.ShuffleOptions(101); err != nil {
				t.Fatalf(
					"ShuffleOptions() err = %v",
					err,
				)
			}

			firstOptions, _ := firstQuestion.GetOptions()
			secondOptions, _ := secondQuestion.GetOptions()
			if len(firstOptions) != len(secondOptions) {
				t.Fatalf(
					"lengths differ: %d vs %d",
					len(firstOptions),
					len(secondOptions),
				)
			}
			for i := range firstOptions {
				if firstOptions[i] != secondOptions[i] {
					t.Fatalf(
						"mismatch at %d: %s vs %s",
						i,
						firstOptions[i],
						secondOptions[i],
					)
				}
			}
		},
	)

	t.Run(
		"preserves all elements",
		func(t *testing.T) {
			t.Parallel()
			question := &Question{ID: 10, Options: json.RawMessage(`["apple","banana","cherry","durian"]`)}
			if err := question.ShuffleOptions(555); err != nil {
				t.Fatalf(
					"ShuffleOptions() err = %v",
					err,
				)
			}
			options, err := question.GetOptions()
			if err != nil {
				t.Fatalf(
					"GetOptions() err = %v",
					err,
				)
			}
			counts := make(map[string]int)
			for _, option := range options {
				counts[option]++
			}
			for _, want := range []string{"apple", "banana", "cherry", "durian"} {
				if counts[want] != 1 {
					t.Fatalf(
						"expected 1 of %s, got %d",
						want,
						counts[want],
					)
				}
			}
		},
	)

	t.Run(
		"produces different orders across different sessions",
		func(t *testing.T) {
			t.Parallel()
			raw := `["1","2","3","4","5"]`
			seenOrders := make(map[string]bool)
			for sessionID := 1; sessionID <= 20; sessionID++ {
				question := &Question{ID: 1, Options: json.RawMessage(raw)}
				if err := question.ShuffleOptions(sessionID); err != nil {
					t.Fatalf(
						"ShuffleOptions() err = %v",
						err,
					)
				}
				seenOrders[string(question.Options)] = true
			}
			if len(seenOrders) <= 1 {
				t.Fatalf(
					"expected multiple different permutations across 20 sessions, got %d",
					len(seenOrders),
				)
			}
		},
	)

	t.Run(
		"noop for empty or single element",
		func(t *testing.T) {
			t.Parallel()
			emptyQuestion := &Question{ID: 1, Options: json.RawMessage(`[]`)}
			if err := emptyQuestion.ShuffleOptions(1); err != nil {
				t.Fatalf(
					"ShuffleOptions() empty err = %v",
					err,
				)
			}
			if string(emptyQuestion.Options) != `[]` {
				t.Fatalf(
					"expected unchanged empty options, got %s",
					string(emptyQuestion.Options),
				)
			}

			singleQuestion := &Question{ID: 1, Options: json.RawMessage(`["only"]`)}
			if err := singleQuestion.ShuffleOptions(1); err != nil {
				t.Fatalf(
					"ShuffleOptions() single err = %v",
					err,
				)
			}
			if string(singleQuestion.Options) != `["only"]` {
				t.Fatalf(
					"expected unchanged single options, got %s",
					string(singleQuestion.Options),
				)
			}

			nilQuestion := &Question{ID: 1}
			if err := nilQuestion.ShuffleOptions(1); err != nil {
				t.Fatalf(
					"ShuffleOptions() nil err = %v",
					err,
				)
			}
		},
	)

	t.Run(
		"invalid JSON returns error",
		func(t *testing.T) {
			t.Parallel()
			invalidQuestion := &Question{ID: 1, Options: json.RawMessage(`{not-json}`)}
			if err := invalidQuestion.ShuffleOptions(1); err == nil {
				t.Fatal("expected error on invalid JSON, got nil")
			}
		},
	)
}

func TestSkillPointer(t *testing.T) {
	skill := SkillPointer(SkillVocabularyContext)
	if skill == nil || *skill != SkillVocabularyContext {
		t.Fatalf(
			"SkillPointer = %v, want %q",
			skill,
			SkillVocabularyContext,
		)
	}
}

func TestSkillTaxonomyIncludesN1Types(t *testing.T) {
	tests := []Skill{
		SkillKanjiReading,
		SkillVocabularyKanjiRecall,
		SkillVocabularyContext,
		SkillVocabularyParaphrase,
		SkillVocabularyUsage,
		SkillGrammarForm,
		SkillSentenceComposition,
		SkillTextGrammar,
		SkillReadingShort,
		SkillReadingMid,
		SkillReadingLong,
		SkillReadingIntegrated,
		SkillReadingThematic,
		SkillInformationRetrieval,
		SkillListeningTask,
		SkillListeningKeyPoint,
		SkillListeningOutline,
		SkillListeningQuickResponse,
		SkillListeningIntegrated,
	}

	for _, skill := range tests {
		if skill == "" {
			t.Fatal("skill must not be empty")
		}
	}
}
