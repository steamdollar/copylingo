package repository

import (
	"context"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"

	"github.com/lsj/copylingo/internal/model"
)

const temporaryMaterialPreferencesTable = `CREATE TEMP TABLE user_material_preferences (
	user_id bigint NOT NULL, material_id integer NOT NULL,
	review_mode text NOT NULL CHECK (review_mode IN ('maintenance', 'excluded')),
	next_check_at timestamptz, check_interval_days integer NOT NULL DEFAULT 30 CHECK (check_interval_days BETWEEN 30 AND 180),
	created_at timestamptz NOT NULL DEFAULT NOW(), updated_at timestamptz NOT NULL DEFAULT NOW(),
	PRIMARY KEY (user_id, material_id), CHECK ((review_mode = 'maintenance') = (next_check_at IS NOT NULL))
) ON COMMIT DROP`

const temporaryPreferenceQuestionsTable = `CREATE TEMP TABLE questions (
	id integer PRIMARY KEY, question_key text, content_id integer, material_id integer,
	type text NOT NULL DEFAULT 'multiple_choice', item_type text,
	language text NOT NULL DEFAULT 'ja', proficiency_level text NOT NULL DEFAULT 'N4',
	category text NOT NULL DEFAULT 'vocabulary', prompt text NOT NULL DEFAULT 'test',
	options jsonb NOT NULL DEFAULT '[]', correct_answer text NOT NULL DEFAULT 'answer',
	explanation text NOT NULL DEFAULT '', audio_path text, audio_script text, audio_file_id text,
	difficulty integer NOT NULL DEFAULT 1, created_at timestamptz NOT NULL DEFAULT NOW()
) ON COMMIT DROP`

func preferenceTestTransaction(t *testing.T) *sqlx.Tx {
	t.Helper()
	dsn := os.Getenv("COPYLINGO_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("COPYLINGO_TEST_DATABASE_URL is not set")
	}
	db, err := sqlx.Open("postgres", dsn)
	if err != nil {
		t.Fatal("open test database failed")
	}
	t.Cleanup(func() { db.Close() })
	tx, err := db.BeginTxx(context.Background(), nil)
	if err != nil {
		t.Fatal("begin test transaction failed")
	}
	t.Cleanup(func() { tx.Rollback() })
	for _, statement := range []string{
		temporaryMaterialPreferencesTable,
		temporaryPreferenceQuestionsTable,
		`CREATE TEMP TABLE materials (
			id integer PRIMARY KEY, material_key text NOT NULL, content_id integer,
			category text NOT NULL DEFAULT 'vocabulary', language text NOT NULL DEFAULT 'ja',
			proficiency_level text NOT NULL DEFAULT 'N4', title text NOT NULL DEFAULT 'title',
			payload jsonb NOT NULL DEFAULT '{}', difficulty integer NOT NULL DEFAULT 1,
			created_at timestamptz NOT NULL DEFAULT NOW()
		) ON COMMIT DROP`,
		`CREATE TEMP TABLE user_material_progress (
			user_id bigint NOT NULL, material_id integer NOT NULL,
			ease_factor double precision NOT NULL DEFAULT 2.5, interval_days integer NOT NULL DEFAULT 3,
			repetitions integer NOT NULL DEFAULT 1, next_review_at timestamptz, last_studied_at timestamptz,
			times_studied integer NOT NULL DEFAULT 1, updated_at timestamptz NOT NULL DEFAULT NOW(),
			PRIMARY KEY (user_id, material_id)
		) ON COMMIT DROP`,
		`CREATE TEMP TABLE user_question_progress (
			user_id bigint NOT NULL, question_id integer NOT NULL, next_review_at timestamptz,
			times_served integer NOT NULL DEFAULT 1, times_correct integer NOT NULL DEFAULT 1,
			ease_factor double precision NOT NULL DEFAULT 2.5, interval_days integer NOT NULL DEFAULT 3,
			repetitions integer NOT NULL DEFAULT 1, last_reviewed_at timestamptz,
			updated_at timestamptz NOT NULL DEFAULT NOW(), PRIMARY KEY (user_id, question_id)
		) ON COMMIT DROP`,
		`CREATE TEMP TABLE sessions (
			id integer PRIMARY KEY, user_id bigint NOT NULL, mode text NOT NULL DEFAULT 'quiz',
			status text NOT NULL DEFAULT 'pending', correct_count integer NOT NULL DEFAULT 0,
			created_at timestamptz NOT NULL DEFAULT NOW(), completed_at timestamptz
		) ON COMMIT DROP`,
		`CREATE TEMP TABLE session_questions (
			id integer PRIMARY KEY, session_id integer NOT NULL, question_id integer NOT NULL,
			user_answer text, is_correct boolean
		) ON COMMIT DROP`,
		`CREATE TEMP TABLE session_materials (
			id integer PRIMARY KEY, session_id integer NOT NULL, material_id integer NOT NULL, studied_at timestamptz
		) ON COMMIT DROP`,
	} {
		mustPreferenceExec(t, tx, statement)
	}
	return tx
}

func mustPreferenceExec(t *testing.T, tx *sqlx.Tx, query string, args ...any) {
	t.Helper()
	if _, err := tx.ExecContext(context.Background(), query, args...); err != nil {
		t.Fatal(err)
	}
}

func TestMaterialPreferenceRepositoryPostgres(t *testing.T) {
	tx := preferenceTestTransaction(t)
	ctx := context.Background()
	repo := &MaterialPreferenceRepository{db: tx}
	mustPreferenceExec(
		t,
		tx,
		`INSERT INTO materials (id, material_key, title) VALUES (1, 'one', 'First'), (2, 'two', 'Second')`,
	)
	mustPreferenceExec(
		t,
		tx,
		`INSERT INTO user_material_progress (user_id, material_id, next_review_at) VALUES (42, 1, NOW() - INTERVAL '2 days')`,
	)
	mustPreferenceExec(t, tx, `INSERT INTO questions (id, material_id) VALUES (1, 1), (2, 2)`)
	mustPreferenceExec(
		t,
		tx,
		`INSERT INTO user_question_progress (user_id, question_id, times_served, times_correct) VALUES (42, 1, 9, 7)`,
	)
	var historyBefore, historyAfter string
	const historyQuery = `SELECT (SELECT jsonb_agg(to_jsonb(p)) FROM user_material_progress p)::text || (SELECT jsonb_agg(to_jsonb(p)) FROM user_question_progress p)::text`
	if err := tx.GetContext(ctx, &historyBefore, historyQuery); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(ctx, 42, 1)
	if err != nil || got.ReviewMode != model.MaterialReviewNormal {
		t.Fatalf("absent preference = %+v, %v", got, err)
	}
	for _, id := range []int{1, 2} {
		if err := repo.Set(ctx, 42, id, model.MaterialReviewMaintenance); err != nil {
			t.Fatal(err)
		}
	}
	got, err = repo.Get(ctx, 42, 1)
	if err != nil || got.CheckIntervalDays != 30 || got.NextCheckAt == nil {
		t.Fatalf("maintenance = %+v, %v", got, err)
	}
	if until := time.Until(*got.NextCheckAt); until < 29*24*time.Hour || until > 31*24*time.Hour {
		t.Fatalf("initial check interval = %s", until)
	}
	mustPreferenceExec(
		t,
		tx,
		`UPDATE user_material_preferences SET check_interval_days = 120, next_check_at = NOW() - INTERVAL '1 day', updated_at = NOW() - INTERVAL '2 days' WHERE user_id = 42 AND material_id = 1`,
	)
	before, err := repo.Get(ctx, 42, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Set(ctx, 42, 1, model.MaterialReviewMaintenance); err != nil {
		t.Fatal(err)
	}
	after, err := repo.Get(ctx, 42, 1)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("same-mode write changed schedule: before=%+v after=%+v err=%v", before, after, err)
	}
	if err := repo.Set(ctx, 99, 1, model.MaterialReviewExcluded); err != nil {
		t.Fatal(err)
	}
	list, err := repo.List(ctx, 42, 1, 1)
	if err != nil || len(list) != 1 || list[0].MaterialID != 2 || list[0].MaterialTitle != "Second" {
		t.Fatalf("paged list = %+v, %v", list, err)
	}
	for _, id := range []int{1, 2} {
		if err := repo.Set(ctx, 42, id, model.MaterialReviewExcluded); err != nil {
			t.Fatal(err)
		}
		excluded, err := repo.Get(ctx, 42, id)
		if err != nil || excluded.NextCheckAt != nil {
			t.Fatalf("excluded = %+v, %v", excluded, err)
		}
		if err := repo.Set(ctx, 42, id, model.MaterialReviewNormal); err != nil {
			t.Fatal(err)
		}
	}
	other, err := repo.Get(ctx, 99, 1)
	if err != nil || other.ReviewMode != model.MaterialReviewExcluded {
		t.Fatalf("other user changed: %+v, %v", other, err)
	}
	if err := tx.GetContext(ctx, &historyAfter, historyQuery); err != nil {
		t.Fatal(err)
	}
	if historyBefore != historyAfter {
		t.Fatal("setting modes changed learning history")
	}
	var unseen int
	if err := tx.GetContext(
		ctx,
		&unseen,
		`SELECT COUNT(*) FROM user_material_progress WHERE user_id=42 AND material_id=2`,
	); err != nil ||
		unseen != 0 {
		t.Fatalf("unseen material gained history: %d, %v", unseen, err)
	}
	var questions []model.Question
	if err := tx.SelectContext(
		ctx,
		&questions,
		newQuestionsForStudiedMaterialsQuery,
		42,
		"ja",
		pq.Array([]string{"N4"}),
		"vocabulary",
		pq.Array([]int{}),
		10,
		3,
	); err != nil {
		t.Fatal(err)
	}
	if len(questions) != 1 || questions[0].ID != 2 {
		t.Fatalf("normal did not restore unseen quiz: %+v", questions)
	}
	if err := repo.Set(ctx, 42, 1, "invalid"); err == nil {
		t.Fatal("invalid mode accepted")
	}
}

func TestMaterialPreferenceSelectionPostgres(t *testing.T) {
	tx := preferenceTestTransaction(t)
	ctx := context.Background()
	for _, query := range []string{
		`INSERT INTO materials (id, material_key) SELECT id, 'material-' || id FROM generate_series(1,12) id`,
		`UPDATE materials SET category='reading' WHERE id IN (6,12)`,
		`INSERT INTO user_material_preferences (user_id, material_id, review_mode, next_check_at)
		 SELECT 42, id, 'maintenance', NOW() - INTERVAL '1 day' FROM materials WHERE id BETWEEN 2 AND 12 AND id NOT IN (3,9)`,
		`UPDATE user_material_preferences SET next_check_at=NOW()+INTERVAL '1 day' WHERE material_id=2`,
		`INSERT INTO user_material_preferences (user_id, material_id, review_mode) VALUES (42,3,'excluded'),(99,9,'excluded')`,
		`INSERT INTO user_material_progress (user_id,material_id,next_review_at,last_studied_at)
		 SELECT 42,id,NOW()-INTERVAL '1 day',NOW()-INTERVAL '3 days' FROM materials WHERE id IN (4,8,11,12)`,
		`UPDATE user_material_progress SET next_review_at=NOW()+INTERVAL '1 day' WHERE material_id=8`,
		`INSERT INTO questions (id,material_id) VALUES
		 (10,1),(11,1),(20,2),(21,2),(30,3),(31,3),(40,4),(41,4),(42,4),(43,4),
		 (60,6),(70,7),(90,9),(100,10),(101,10),(102,10),(110,11),(120,12),(130,NULL)`,
		`UPDATE questions SET category='reading' WHERE id IN (60,120)`,
		`UPDATE questions SET category='listening' WHERE id=70`,
		`UPDATE questions SET category='grammar' WHERE id=42`,
		`UPDATE questions SET item_type='vocab_kanji_recall' WHERE id=43`,
		`UPDATE questions SET proficiency_level='N3' WHERE id=100`,
		`INSERT INTO user_question_progress (user_id,question_id,next_review_at)
		 SELECT 42,id,NOW()-INTERVAL '1 day' FROM questions WHERE id IN (11,21,31,41,42,43,110,120)`,
		`UPDATE user_question_progress SET next_review_at=NOW()+INTERVAL '1 day' WHERE question_id=110`,
	} {
		mustPreferenceExec(t, tx, query)
	}

	newQuestions := func(userID int64, category string, exclusions []int) []model.Question {
		t.Helper()
		var got []model.Question
		if err := tx.SelectContext(
			ctx,
			&got,
			newQuestionsForStudiedMaterialsQuery,
			userID,
			"ja",
			pq.Array([]string{"N4"}),
			category,
			pq.Array(exclusions),
			100,
			3,
		); err != nil {
			t.Fatal(err)
		}
		return got
	}
	assertIDs := func(got []model.Question, want ...int) {
		t.Helper()
		seen := make(map[int]bool)
		for _, q := range got {
			seen[q.ID] = true
			wantMaintenance := q.ID == 41 || q.ID == 101 || q.ID == 120
			if q.IsMaintenanceCheck != wantMaintenance {
				t.Fatalf(
					"question %d maintenance selection hint = %v, want %v",
					q.ID,
					q.IsMaintenanceCheck,
					wantMaintenance,
				)
			}
		}
		if len(got) != len(want) {
			t.Fatalf("question IDs=%v, want %v", seen, want)
		}
		for _, id := range want {
			if !seen[id] {
				t.Fatalf("question IDs=%v, want %v", seen, want)
			}
		}
	}
	assertIDs(newQuestions(42, "", nil), 10, 90, 101, 130)
	assertIDs(newQuestions(42, "vocabulary", []int{102}), 10, 90, 130)
	assertIDs(newQuestions(42, "grammar", nil))
	// The deterministic representative is in-scope even though its first linked
	// variant belongs to a higher level; a second call cannot choose a variant.
	assertIDs(newQuestions(42, "vocabulary", []int{101}), 10, 90, 130)
	other := newQuestions(99, "vocabulary", nil)
	otherIDs := make(map[int]bool)
	for _, q := range other {
		otherIDs[q.ID] = true
	}
	for _, id := range []int{20, 21, 30, 31, 40, 41, 101, 102} {
		if !otherIDs[id] {
			t.Fatalf("another user's setting/progress blocked question %d: %v", id, otherIDs)
		}
	}
	if otherIDs[90] {
		t.Fatal("own excluded preference was ignored")
	}
	for _, category := range [][]model.QuestionCategory{nil, {model.CategoryVocabulary}, {model.CategoryGrammar}, {model.CategoryReading}} {
		var got []model.Question
		if err := tx.SelectContext(
			ctx,
			&got,
			dueReviewsForStudiedMaterialsQuery,
			42,
			"ja",
			pq.Array([]string{"N4"}),
			100,
			3,
			"N4",
			pq.Array(category),
		); err != nil {
			t.Fatal(err)
		}
		switch {
		case len(category) == 0:
			assertIDs(got, 11, 41, 120)
		case category[0] == model.CategoryVocabulary:
			assertIDs(got, 11, 41)
		case category[0] == model.CategoryGrammar:
			assertIDs(got)
		case category[0] == model.CategoryReading:
			assertIDs(got, 120)
		}
	}
	var count int
	if err := tx.GetContext(ctx, &count, dueReviewCountQuery, 42, "ja", pq.Array([]string{"N4"})); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("gated due count=%d, want 3", count)
	}
	plan := model.StudySessionPlan{Quotas: []model.StudyMaterialQuota{
		{Category: model.MaterialCategoryVocabulary, NewCount: 1, ReviewCount: 20},
		{Category: model.MaterialCategoryReading, NewCount: 1},
	}}
	repo := &MaterialRepository{db: tx}
	got, err := repo.GetMaterialsByPlan(ctx, 42, "ja", "N4", []string{"N4"}, plan)
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[int]bool)
	for _, m := range got {
		seen[m.ID] = true
	}
	want := []int{1, 5, 6, 7, 9, 11}
	if len(got) != len(want) {
		t.Fatalf("study selection=%v, want %v", seen, want)
	}
	for _, id := range want {
		if !seen[id] {
			t.Fatalf("study selection=%v, want %v", seen, want)
		}
	}
	// Empty primary buckets still cannot reintroduce excluded/future material
	// through the due or new-vocabulary top-up paths.
	for _, id := range []int{2, 3, 4, 8, 10, 12} {
		if seen[id] {
			t.Fatalf("study fallback bypassed preference/quiz/SRS gate for %d", id)
		}
	}
}
