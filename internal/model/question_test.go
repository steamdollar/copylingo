package model

import (
	"encoding/json"
	"testing"
)

func TestQuestion_GetOptions(t *testing.T) {
	t.Parallel()

	t.Run("parses JSONB array", func(t *testing.T) {
		t.Parallel()
		q := &Question{Options: json.RawMessage(`["A","B","C","D"]`)}
		opts, err := q.GetOptions()
		if err != nil {
			t.Fatalf("GetOptions() error = %v", err)
		}
		if len(opts) != 4 || opts[0] != "A" || opts[3] != "D" {
			t.Fatalf("GetOptions() = %v, want [A B C D]", opts)
		}
	})

	t.Run("empty array", func(t *testing.T) {
		t.Parallel()
		q := &Question{Options: json.RawMessage(`[]`)}
		opts, err := q.GetOptions()
		if err != nil {
			t.Fatalf("GetOptions() error = %v", err)
		}
		if len(opts) != 0 {
			t.Fatalf("GetOptions() = %v, want empty", opts)
		}
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		t.Parallel()
		q := &Question{Options: json.RawMessage(`{"not":"an array"}`)}
		if _, err := q.GetOptions(); err == nil {
			t.Fatal("GetOptions() error = nil, want unmarshal error")
		}
	})

	t.Run("nil options returns error", func(t *testing.T) {
		t.Parallel()
		q := &Question{}
		if _, err := q.GetOptions(); err == nil {
			t.Fatal("GetOptions() on nil options error = nil, want error")
		}
	})
}

func TestSkillPtr(t *testing.T) {
	skill := SkillPtr(SkillVocabContext)
	if skill == nil || *skill != SkillVocabContext {
		t.Fatalf("SkillPtr = %v, want %q", skill, SkillVocabContext)
	}
}

func TestSkillTaxonomyIncludesN1Types(t *testing.T) {
	tests := []Skill{
		SkillKanjiReading,
		SkillVocabContext,
		SkillVocabParaphrase,
		SkillVocabUsage,
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
