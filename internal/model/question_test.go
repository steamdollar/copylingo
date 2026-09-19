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

func TestQuestion_ShuffleOptions(t *testing.T) {
	t.Parallel()

	t.Run("deterministic for same session and question ID", func(t *testing.T) {
		t.Parallel()
		raw := `["optA","optB","optC","optD"]`
		q1 := &Question{ID: 42, Options: json.RawMessage(raw)}
		q2 := &Question{ID: 42, Options: json.RawMessage(raw)}

		if err := q1.ShuffleOptions(101); err != nil {
			t.Fatalf("ShuffleOptions() err = %v", err)
		}
		if err := q2.ShuffleOptions(101); err != nil {
			t.Fatalf("ShuffleOptions() err = %v", err)
		}

		opts1, _ := q1.GetOptions()
		opts2, _ := q2.GetOptions()
		if len(opts1) != len(opts2) {
			t.Fatalf("lengths differ: %d vs %d", len(opts1), len(opts2))
		}
		for i := range opts1 {
			if opts1[i] != opts2[i] {
				t.Fatalf("mismatch at %d: %s vs %s", i, opts1[i], opts2[i])
			}
		}
	})

	t.Run("preserves all elements", func(t *testing.T) {
		t.Parallel()
		q := &Question{ID: 10, Options: json.RawMessage(`["apple","banana","cherry","durian"]`)}
		if err := q.ShuffleOptions(555); err != nil {
			t.Fatalf("ShuffleOptions() err = %v", err)
		}
		opts, err := q.GetOptions()
		if err != nil {
			t.Fatalf("GetOptions() err = %v", err)
		}
		counts := make(map[string]int)
		for _, o := range opts {
			counts[o]++
		}
		for _, want := range []string{"apple", "banana", "cherry", "durian"} {
			if counts[want] != 1 {
				t.Fatalf("expected 1 of %s, got %d", want, counts[want])
			}
		}
	})

	t.Run("produces different orders across different sessions", func(t *testing.T) {
		t.Parallel()
		raw := `["1","2","3","4","5"]`
		seenOrders := make(map[string]bool)
		for sessionID := 1; sessionID <= 20; sessionID++ {
			q := &Question{ID: 1, Options: json.RawMessage(raw)}
			if err := q.ShuffleOptions(sessionID); err != nil {
				t.Fatalf("ShuffleOptions() err = %v", err)
			}
			seenOrders[string(q.Options)] = true
		}
		if len(seenOrders) <= 1 {
			t.Fatalf("expected multiple different permutations across 20 sessions, got %d", len(seenOrders))
		}
	})

	t.Run("noop for empty or single element", func(t *testing.T) {
		t.Parallel()
		qEmpty := &Question{ID: 1, Options: json.RawMessage(`[]`)}
		if err := qEmpty.ShuffleOptions(1); err != nil {
			t.Fatalf("ShuffleOptions() empty err = %v", err)
		}
		if string(qEmpty.Options) != `[]` {
			t.Fatalf("expected unchanged empty options, got %s", string(qEmpty.Options))
		}

		qSingle := &Question{ID: 1, Options: json.RawMessage(`["only"]`)}
		if err := qSingle.ShuffleOptions(1); err != nil {
			t.Fatalf("ShuffleOptions() single err = %v", err)
		}
		if string(qSingle.Options) != `["only"]` {
			t.Fatalf("expected unchanged single options, got %s", string(qSingle.Options))
		}

		qNil := &Question{ID: 1}
		if err := qNil.ShuffleOptions(1); err != nil {
			t.Fatalf("ShuffleOptions() nil err = %v", err)
		}
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		t.Parallel()
		qInvalid := &Question{ID: 1, Options: json.RawMessage(`{not-json}`)}
		if err := qInvalid.ShuffleOptions(1); err == nil {
			t.Fatal("expected error on invalid JSON, got nil")
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
		SkillVocabKanjiRecall,
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
