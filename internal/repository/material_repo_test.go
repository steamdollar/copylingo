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
			MaterialKey:      "ja:vocab:n5_word_024",
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

func TestStudySessionMaterialsQueryIncludesGrammarAndInterleavesCategories(t *testing.T) {
	for _, want := range []string{
		"m.category = ANY($4)",
		"PARTITION BY mp.category",
		"WHEN 'vocabulary' THEN 0",
		"WHEN 'grammar' THEN 1",
		"WHEN 'reading' THEN 2",
		"CASE WHEN category = 'reading' THEN category_bucket_rank ELSE category_rank END ASC",
		"category_order ASC",
	} {
		if !strings.Contains(studySessionMaterialsQuery, want) {
			t.Fatalf("studySessionMaterialsQuery does not contain %q:\n%s", want, studySessionMaterialsQuery)
		}
	}
}

func TestStudySessionMaterialCategoriesIncludeReading(t *testing.T) {
	want := []string{
		string(model.MaterialCategoryVocabulary),
		string(model.MaterialCategoryGrammar),
		string(model.MaterialCategoryReading),
	}
	if len(studySessionMaterialCategories) != len(want) {
		t.Fatalf("studySessionMaterialCategories = %v, want %v", studySessionMaterialCategories, want)
	}
	for i, category := range want {
		if studySessionMaterialCategories[i] != category {
			t.Fatalf("studySessionMaterialCategories = %v, want %v", studySessionMaterialCategories, want)
		}
	}
}

func TestStudySessionMaterialsQueryReadingPolicy(t *testing.T) {
	t.Run("due and new coexist selects at most one from each bucket", func(t *testing.T) {
		for _, want := range []string{
			"CASE WHEN mp.progress_material_id IS NULL THEN 'new' ELSE 'review' END",
			"(progress_material_id IS NULL AND category_bucket_rank <= 1)",
			"CASE WHEN has_unseen_reading THEN 1 ELSE 2 END",
		} {
			if !strings.Contains(studySessionMaterialsQuery, want) {
				t.Fatalf("studySessionMaterialsQuery does not contain %q:\n%s", want, studySessionMaterialsQuery)
			}
		}
	})

	t.Run("all reading seen allows two due reviews", func(t *testing.T) {
		for _, want := range []string{
			"WHERE category = 'reading'",
			"AND progress_material_id IS NULL",
			") AS has_unseen_reading",
			"progress_material_id IS NOT NULL",
			"CASE WHEN has_unseen_reading THEN 1 ELSE 2 END",
		} {
			if !strings.Contains(studySessionMaterialsQuery, want) {
				t.Fatalf("studySessionMaterialsQuery does not contain %q:\n%s", want, studySessionMaterialsQuery)
			}
		}
	})

	t.Run("no due review never pulls a future review forward", func(t *testing.T) {
		for _, want := range []string{
			"WHERE (mp.progress_material_id IS NULL OR mp.next_review_at <= NOW())",
			"(progress_material_id IS NULL AND category_bucket_rank <= 1)",
		} {
			if !strings.Contains(studySessionMaterialsQuery, want) {
				t.Fatalf("studySessionMaterialsQuery does not contain %q:\n%s", want, studySessionMaterialsQuery)
			}
		}
		if strings.Contains(studySessionMaterialsQuery, "mp.next_review_at IS NOT NULL") {
			t.Fatalf("studySessionMaterialsQuery must not admit not-yet-due reviews:\n%s", studySessionMaterialsQuery)
		}
	})

	t.Run("non reading categories and general limit remain unchanged", func(t *testing.T) {
		for _, want := range []string{
			"WHERE category <> 'reading'",
			"CASE WHEN category = 'reading' THEN category_bucket_rank ELSE category_rank END ASC",
			"category_order ASC",
			"LIMIT $5",
		} {
			if !strings.Contains(studySessionMaterialsQuery, want) {
				t.Fatalf("studySessionMaterialsQuery does not contain %q:\n%s", want, studySessionMaterialsQuery)
			}
		}
	})

	t.Run("many due readings do not push selected new reading past the global limit", func(t *testing.T) {
		for _, want := range []string{
			"CASE WHEN category = 'reading' THEN category_bucket_rank ELSE category_rank END ASC",
			"CASE WHEN category = 'reading' AND progress_material_id IS NULL THEN 1 ELSE 0 END ASC",
			"id ASC",
		} {
			if !strings.Contains(studySessionMaterialsQuery, want) {
				t.Fatalf("studySessionMaterialsQuery does not contain %q:\n%s", want, studySessionMaterialsQuery)
			}
		}
		if strings.Contains(studySessionMaterialsQuery, "ORDER BY category_rank ASC, category_order ASC") {
			t.Fatalf(
				"studySessionMaterialsQuery still orders selected reading by the unfiltered category rank:\n%s",
				studySessionMaterialsQuery,
			)
		}
	})

	t.Run("pending and in progress materials remain excluded", func(t *testing.T) {
		for _, want := range []string{
			"AND sm.material_id = mp.id",
			"AND s.mode = 'study'",
			"AND s.status IN ('pending', 'in_progress')",
		} {
			if !strings.Contains(studySessionMaterialsQuery, want) {
				t.Fatalf("studySessionMaterialsQuery does not contain %q:\n%s", want, studySessionMaterialsQuery)
			}
		}
	})
}
