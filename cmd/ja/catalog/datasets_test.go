package catalog

import "testing"

// Integrity regressions for the embedded datasets. The JSON files under data/
// are the source of truth, so these checks catch a malformed or regressed file
// at test time instead of at runtime. They inspect the package vars directly,
// i.e. exactly what the seeder consumes.

func TestN5Grammar_Integrity(t *testing.T) {
	if len(N5GrammarPoints) != 80 {
		t.Fatalf("expected 80 grammar points, got %d", len(N5GrammarPoints))
	}
	seen := make(map[string]bool, len(N5GrammarPoints))
	for i, p := range N5GrammarPoints {
		if p.ID == "" || p.Pattern == "" || p.MeaningKo == "" ||
			p.Example == "" || p.ExampleReading == "" || p.ClozePrompt == "" || p.CorrectAnswer == "" {
			t.Errorf("point %d (%q) has an empty required field", i, p.ID)
		}
		if seen[p.ID] {
			t.Errorf("duplicate grammar ID %q", p.ID)
		}
		seen[p.ID] = true

		if len(p.FormOptions) < 2 {
			t.Errorf("point %q: expected >=2 form options, got %d", p.ID, len(p.FormOptions))
		}
		found := false
		for _, opt := range p.FormOptions {
			if opt == p.CorrectAnswer {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("point %q: correct answer %q not in form options %v", p.ID, p.CorrectAnswer, p.FormOptions)
		}
	}
}

func TestN5Vocab_Integrity(t *testing.T) {
	if len(N5Words) != 540 {
		t.Fatalf("expected 540 vocab words, got %d", len(N5Words))
	}
	seen := make(map[string]bool, len(N5Words))
	for i, w := range N5Words {
		if w.ID == "" || w.Kana == "" || w.Kanji == "" || w.MeaningKo == "" || w.PartOfSpeech == "" {
			t.Errorf("word %d (%q) has an empty required field", i, w.ID)
		}
		if seen[w.ID] {
			t.Errorf("duplicate vocab ID %q", w.ID)
		}
		seen[w.ID] = true
	}
}

func TestKana_Integrity(t *testing.T) {
	if len(KanaMap) != 208 {
		t.Fatalf("expected 208 kana entries, got %d", len(KanaMap))
	}
	for k, romaji := range KanaMap {
		if k == "" || romaji == "" {
			t.Errorf("kana entry %q→%q has an empty key or value", k, romaji)
		}
	}
}
