package repository

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

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

func TestValidateStudySessionPlan(t *testing.T) {
	valid := model.StudySessionPlan{Quotas: []model.StudyMaterialQuota{
		{Category: model.MaterialCategoryVocabulary, NewCount: 2, ReviewCount: 1},
		{Category: model.MaterialCategoryGrammar, NewCount: 1, ReviewCount: 0},
	}}
	if err := validateStudySessionPlan(valid); err != nil {
		t.Fatalf("validateStudySessionPlan(valid) error = %v", err)
	}

	for name, plan := range map[string]model.StudySessionPlan{
		"negative": {Quotas: []model.StudyMaterialQuota{{
			Category: model.MaterialCategoryVocabulary, NewCount: -1,
		}}},
		"unsupported": {Quotas: []model.StudyMaterialQuota{{
			Category: model.MaterialCategoryKana, NewCount: 1,
		}}},
		"duplicate": {Quotas: []model.StudyMaterialQuota{
			{Category: model.MaterialCategoryVocabulary, NewCount: 1},
			{Category: model.MaterialCategoryVocabulary, ReviewCount: 1},
		}},
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateStudySessionPlan(plan); err == nil {
				t.Fatalf("validateStudySessionPlan(%s) error = nil", name)
			}
		})
	}
}

func TestStudySessionMaterialsQueryUsesQuotaBucketsAndFallbacks(t *testing.T) {
	for _, want := range []string{
		"jsonb_to_recordset($5::jsonb)",
		"m.proficiency_level = ANY($4)",
		"m.proficiency_level = $3",
		"PARTITION BY mp.category, mp.bucket",
		"WHEN 'vocabulary' THEN 0",
		"WHEN 'grammar' THEN 1",
		"WHEN 'reading' THEN 2",
		"r.bucket = 'new' AND r.bucket_rank <= q.new_count",
		"r.bucket = 'review' AND r.bucket_rank <= q.review_count",
		"same_category_due",
		"rd.category IN ('vocabulary', 'grammar')",
		"LIMIT $6",
		"category_order ASC",
	} {
		if !strings.Contains(studySessionMaterialsQuery, want) {
			t.Fatalf("studySessionMaterialsQuery does not contain %q:\n%s", want, studySessionMaterialsQuery)
		}
	}
}

func TestStudySessionMaterialsQueryPolicyGuards(t *testing.T) {
	for _, want := range []string{
		"ump.next_review_at <= NOW()",
		"AND sm.material_id = m.id",
		"AND s.mode = 'study'",
		"AND s.status IN ('pending', 'in_progress')",
		"(rd.category <> 'reading' OR rd.primary_count < rd.category_total)",
		"CASE WHEN s.bucket = 'new' THEN 0 ELSE 1 END",
	} {
		if !strings.Contains(studySessionMaterialsQuery, want) {
			t.Fatalf("studySessionMaterialsQuery does not contain %q:\n%s", want, studySessionMaterialsQuery)
		}
	}
	if strings.Contains(studySessionMaterialsQuery, "LIMIT $5") {
		t.Fatalf("studySessionMaterialsQuery still uses the old limit placeholder:\n%s", studySessionMaterialsQuery)
	}
}

// This test is opt-in because the default repository tests do not require a
// running database. It uses only temporary tables inside a rolled-back
// transaction, so it cannot alter the configured database.
func TestGetForStudySessionPostgres(t *testing.T) {
	dsn := os.Getenv("COPYLINGO_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("COPYLINGO_TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()
	db, err := sqlx.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open test database failed")
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping test database failed")
	}
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		t.Fatalf("begin test transaction failed")
	}
	defer tx.Rollback()

	for _, statement := range []string{
		`CREATE TEMP TABLE materials (
			id integer PRIMARY KEY, material_key text NOT NULL, content_id integer,
			category text NOT NULL, language text NOT NULL, proficiency_level text NOT NULL,
			title text NOT NULL, payload jsonb NOT NULL, difficulty integer NOT NULL,
			created_at timestamptz NOT NULL DEFAULT NOW()
		) ON COMMIT DROP`,
		`CREATE TEMP TABLE user_material_progress (
			user_id bigint NOT NULL, material_id integer NOT NULL,
			next_review_at timestamptz, PRIMARY KEY (user_id, material_id)
		) ON COMMIT DROP`,
		`CREATE TEMP TABLE sessions (
			id integer PRIMARY KEY, user_id bigint NOT NULL, mode text NOT NULL, status text NOT NULL
		) ON COMMIT DROP`,
		`CREATE TEMP TABLE session_materials (session_id integer NOT NULL, material_id integer NOT NULL) ON COMMIT DROP`,
		`INSERT INTO materials (id, material_key, category, language, proficiency_level, title, payload, difficulty) VALUES
			(1, 'v1', 'vocabulary', 'ja', 'N4', 'v1', '{}', 1),
			(2, 'v2', 'vocabulary', 'ja', 'N4', 'v2', '{}', 1),
			(3, 'v3', 'vocabulary', 'ja', 'N4', 'v3', '{}', 1),
			(4, 'v4', 'vocabulary', 'ja', 'N5', 'v4', '{}', 1),
			(5, 'v5', 'vocabulary', 'ja', 'N4', 'v5', '{}', 1),
			(6, 'v6', 'vocabulary', 'ja', 'N4', 'v6', '{}', 1),
			(7, 'v7', 'vocabulary', 'ja', 'N4', 'v7', '{}', 1),
			(8, 'g1', 'grammar', 'ja', 'N4', 'g1', '{}', 1),
			(10, 'g3', 'grammar', 'ja', 'N4', 'g3', '{}', 1),
			(12, 'r1', 'reading', 'ja', 'N4', 'r1', '{}', 1),
			(13, 'r2', 'reading', 'ja', 'N4', 'r2', '{}', 1),
			(14, 'r3', 'reading', 'ja', 'N5', 'r3', '{}', 1),
			(16, 'g4', 'grammar', 'ja', 'N4', 'g4', '{}', 1)`,
		`INSERT INTO user_material_progress (user_id, material_id, next_review_at) VALUES
			(42, 5, NOW() - INTERVAL '3 days'),
			(42, 6, NOW() - INTERVAL '2 days'),
			(42, 7, NOW() + INTERVAL '2 days'),
			(42, 10, NOW() - INTERVAL '1 day'),
			(42, 13, NOW() - INTERVAL '1 day'),
			(42, 16, NOW() - INTERVAL '1 day')`,
		`INSERT INTO sessions (id, user_id, mode, status) VALUES
			(100, 42, 'study', 'pending'), (101, 42, 'study', 'in_progress')`,
		`INSERT INTO session_materials (session_id, material_id) VALUES (100, 2), (101, 14)`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			t.Fatalf("fixture setup failed")
		}
	}

	repo := &MaterialRepository{db: tx}
	levels := []string{"N5", "N4"}
	morning := model.StudySessionPlan{Quotas: []model.StudyMaterialQuota{
		{Category: model.MaterialCategoryVocabulary, NewCount: 2, ReviewCount: 2},
		{Category: model.MaterialCategoryGrammar, NewCount: 1, ReviewCount: 1},
		{Category: model.MaterialCategoryReading, NewCount: 1},
	}}
	got, err := repo.GetForStudySession(ctx, 42, "ja", "N4", levels, morning)
	if err != nil {
		t.Fatalf("morning selection failed: %v", err)
	}
	if len(got) != morning.TotalMaterialCount() {
		t.Fatalf("morning result count = %d, want %d", len(got), morning.TotalMaterialCount())
	}
	gotIDs := make(map[int]model.Material)
	for _, material := range got {
		gotIDs[material.ID] = material
	}
	for _, id := range []int{1, 3, 5, 6, 8, 10, 12} {
		if _, ok := gotIDs[id]; !ok {
			t.Fatalf("morning result missing expected material %d: %#v", id, gotIDs)
		}
	}
	for _, id := range []int{2, 7, 14, 16} {
		if _, ok := gotIDs[id]; ok {
			t.Fatalf("morning result included excluded material %d", id)
		}
	}
	if gotIDs[1].ProficiencyLevel != "N4" || gotIDs[3].ProficiencyLevel != "N4" {
		t.Fatalf("new vocabulary did not prefer current level: %#v", gotIDs)
	}

	evening := model.StudySessionPlan{Quotas: []model.StudyMaterialQuota{
		{Category: model.MaterialCategoryVocabulary, NewCount: 4, ReviewCount: 14},
		{Category: model.MaterialCategoryGrammar, NewCount: 1, ReviewCount: 3},
		{Category: model.MaterialCategoryReading, ReviewCount: 2},
	}}
	eveningMaterials, err := repo.GetForStudySession(ctx, 42, "ja", "N4", levels, evening)
	if err != nil {
		t.Fatalf("evening selection failed: %v", err)
	}
	readingCount := 0
	for _, material := range eveningMaterials {
		if material.Category == model.MaterialCategoryReading {
			readingCount++
			if material.ID == 12 || material.ID == 14 {
				t.Fatalf("evening selection included new reading material %d", material.ID)
			}
		}
	}
	if readingCount > 2 {
		t.Fatalf("evening selection exceeded reading cap: %d", readingCount)
	}

	fallbackPlan := model.StudySessionPlan{Quotas: []model.StudyMaterialQuota{
		{Category: model.MaterialCategoryVocabulary, NewCount: 4},
	}}
	fallback, err := repo.GetForStudySession(ctx, 42, "ja", "N4", levels, fallbackPlan)
	if err != nil {
		t.Fatalf("same-category fallback failed: %v", err)
	}
	if len(fallback) != 4 || fallback[0].ID == 2 {
		t.Fatalf("same-category fallback result = %#v", fallback)
	}
	newCount := 0
	for _, material := range fallback {
		if material.ID == 1 || material.ID == 3 || material.ID == 4 {
			newCount++
		}
	}
	if newCount > fallbackPlan.Quotas[0].NewCount {
		t.Fatalf("same-category fallback exceeded new quota: %d", newCount)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM user_material_progress WHERE material_id IN (10, 16)`); err != nil {
		t.Fatalf("fallback fixture update failed")
	}
	otherDuePlan := model.StudySessionPlan{Quotas: []model.StudyMaterialQuota{
		{Category: model.MaterialCategoryVocabulary, ReviewCount: 0},
		{Category: model.MaterialCategoryGrammar, NewCount: 2},
	}}
	otherDue, err := repo.GetForStudySession(ctx, 42, "ja", "N4", levels, otherDuePlan)
	if err != nil {
		t.Fatalf("other-category fallback failed: %v", err)
	}
	if len(otherDue) != 2 {
		t.Fatalf("other-category fallback result count = %d, want 2", len(otherDue))
	}
	for _, material := range otherDue {
		if material.ID == 1 || material.ID == 3 || material.ID == 4 {
			t.Fatalf("other-category fallback added new material: %#v", otherDue)
		}
	}

	dueOnlyPlan := model.StudySessionPlan{Quotas: []model.StudyMaterialQuota{
		{Category: model.MaterialCategoryVocabulary, ReviewCount: 10},
	}}
	dueOnly, err := repo.GetForStudySession(ctx, 42, "ja", "N4", levels, dueOnlyPlan)
	if err != nil {
		t.Fatalf("due-only selection failed: %v", err)
	}
	for _, material := range dueOnly {
		if material.ID == 1 || material.ID == 3 || material.ID == 4 {
			t.Fatalf("due shortage inflated new workload: %#v", dueOnly)
		}
	}
}
