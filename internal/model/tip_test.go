package model

import "testing"

func TestTipCategory_DisplayName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		category TipCategory
		want     string
	}{
		{"youon", TipCategoryKanaYoon, "요음"},
		{"sokuon", TipCategoryKanaSokuon, "촉음"},
		{"dakuten", TipCategoryKanaDakuten, "탁점/반탁점"},
		{"chouon", TipCategoryKanaChouon, "장음"},
		{"shape", TipCategoryKanaShape, "비슷한 모양"},
		{"stroke", TipCategoryKanaStroke, "획순"},
		{"hira_vs_kata", TipCategoryKanaHiraKata, "히라가나/가타카나"},
		// drift guard: an unknown category falls back to its raw enum string.
		{"unknown fallback", TipCategory("kana_unknown"), "kana_unknown"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.category.DisplayName(); got != tt.want {
				t.Fatalf("TipCategory(%q).DisplayName() = %q, want %q", tt.category, got, tt.want)
			}
		})
	}
}

func TestAllTipCategories(t *testing.T) {
	t.Parallel()

	cats := AllTipCategories()
	if len(cats) != 7 {
		t.Fatalf("AllTipCategories() len = %d, want 7", len(cats))
	}

	// Every returned category must be whitelisted (resolvable to a display name
	// other than the raw fallback). This guards against drift between the map
	// and the exported list.
	seen := make(map[TipCategory]bool, len(cats))
	for _, c := range cats {
		if c.DisplayName() == string(c) {
			t.Fatalf("AllTipCategories() returned non-whitelisted category %q", c)
		}
		if seen[c] {
			t.Fatalf("AllTipCategories() returned duplicate category %q", c)
		}
		seen[c] = true
	}

	for _, want := range []TipCategory{
		TipCategoryKanaYoon, TipCategoryKanaSokuon, TipCategoryKanaDakuten,
		TipCategoryKanaChouon, TipCategoryKanaShape, TipCategoryKanaStroke,
		TipCategoryKanaHiraKata,
	} {
		if !seen[want] {
			t.Fatalf("AllTipCategories() missing %q", want)
		}
	}
}
