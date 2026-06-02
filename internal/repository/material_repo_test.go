package repository

import (
	"strings"
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

func TestBuildMaterialBatchUpsertQuery(t *testing.T) {
	materials := []*model.Material{
		{
			MaterialKey:      "ja:kana:u3042",
			Category:         model.MaterialCategoryKana,
			Language:         "ja",
			ProficiencyLevel: "N5",
			Title:            "あ",
			Payload:          []byte(`{"kana":"あ","romaji":"a"}`),
			Difficulty:       1,
		},
		{
			MaterialKey:      "ja:vocab:word_024",
			Category:         model.MaterialCategoryVocabulary,
			Language:         "ja",
			ProficiencyLevel: "N5",
			Title:            "みず",
			Payload:          []byte(`{"kana":"みず","meaning_ko":"물"}`),
			Difficulty:       2,
		},
	}

	query, args := buildMaterialBatchUpsertQuery(materials)

	if !strings.Contains(query, "INSERT INTO materials") {
		t.Fatalf("query = %q, want insert statement", query)
	}
	if !strings.Contains(query, "($1, $2, $3, $4, $5, $6, $7, $8)") {
		t.Fatalf("query = %q, want first placeholder group", query)
	}
	if !strings.Contains(query, "($9, $10, $11, $12, $13, $14, $15, $16)") {
		t.Fatalf("query = %q, want second placeholder group", query)
	}
	if !strings.Contains(query, "ON CONFLICT (material_key) DO UPDATE") {
		t.Fatalf("query = %q, want material key upsert", query)
	}
	if len(args) != 16 {
		t.Fatalf("len(args) = %d, want 16", len(args))
	}
}
