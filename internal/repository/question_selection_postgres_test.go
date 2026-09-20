package repository

import (
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"

	"github.com/lsj/copylingo/internal/model"
)

// Selection policies need real SQL execution; all fixtures live in temporary
// tables in a rolled-back transaction, never in the application's tables.
func TestQuestionSelectionPostgres(t *testing.T) {
	dsn := os.Getenv("COPYLINGO_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("COPYLINGO_TEST_DATABASE_URL is not set")
	}
	db, err := sqlx.Open("postgres", dsn)
	if err != nil {
		t.Fatal("open test database failed")
	}
	defer db.Close()
	ctx := context.Background()
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		t.Fatal("begin test transaction failed")
	}
	defer tx.Rollback()
	for _, statement := range []string{
		`CREATE TEMP TABLE questions (
			id integer PRIMARY KEY, question_key text, content_id integer, material_id integer,
			type text NOT NULL DEFAULT 'multiple_choice', item_type text,
			language text NOT NULL DEFAULT 'ja', proficiency_level text NOT NULL DEFAULT 'N4',
			category text NOT NULL DEFAULT 'vocabulary', prompt text NOT NULL DEFAULT 'test',
			options jsonb NOT NULL DEFAULT '[]', correct_answer text NOT NULL DEFAULT 'answer',
			explanation text NOT NULL DEFAULT '', audio_path text, audio_script text, audio_file_id text,
			difficulty integer NOT NULL DEFAULT 1, created_at timestamptz NOT NULL DEFAULT NOW()
		) ON COMMIT DROP`,
		`CREATE TEMP TABLE user_material_progress (
			user_id bigint, material_id integer, times_studied integer NOT NULL DEFAULT 1,
			last_studied_at timestamptz, PRIMARY KEY (user_id, material_id)
		) ON COMMIT DROP`,
		`CREATE TEMP TABLE user_question_progress (
			user_id bigint, question_id integer, next_review_at timestamptz,
			PRIMARY KEY (user_id, question_id)
		) ON COMMIT DROP`,
		`INSERT INTO user_material_progress (user_id, material_id, last_studied_at) VALUES
			(42, 100, NOW() - INTERVAL '1 hour'), (42, 200, NOW() - INTERVAL '10 days'),
			(42, 300, NOW() - INTERVAL '1 day'), (99, 500, NOW()),
			(42, 600, NOW()), (42, 700, NOW() - INTERVAL '1 day'), (42, 800, NOW() - INTERVAL '2 days')`,
		`INSERT INTO questions (id, material_id, difficulty) VALUES
			(1, 100, 2), (2, 100, 3), (3, 200, 1), (4, 300, 3), (5, 400, 1),
			(6, NULL, 1), (10, 300, 1), (15, 500, 1)`,
		`INSERT INTO questions (id, material_id, language, proficiency_level) VALUES
			(8, 100, 'ko', 'N4'), (9, 100, 'ja', 'N5')`,
		`INSERT INTO questions (id, category, material_id, audio_path) VALUES
			(11, 'listening', NULL, NULL), (12, 'listening', NULL, 'ready.ogg'),
			(13, 'reading', 100, NULL), (14, 'reading', 400, NULL), (16, 'reading', 500, NULL)`,
		`INSERT INTO questions (id, material_id, item_type) VALUES
			(30, 600, 'vocab_kanji_recall'), (31, 700, 'vocab_kanji_recall'), (32, 800, 'vocab_kanji_recall')`,
		`INSERT INTO user_question_progress (user_id, question_id) VALUES (42, 10)`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}

	for _, tt := range []struct {
		name     string
		category string
		exclude  []int
		limit    int
		wantIDs  []int
	}{
		{"recent study and material diversity precede difficulty", "vocabulary", nil, 4, []int{1, 4, 3, 2}},
		{"material diversity includes earlier selections", "vocabulary", []int{1}, 3, []int{4, 3, 2}},
		{"reading requires this user's study", "reading", nil, 10, []int{13}},
		{"listening requires audio", "listening", nil, 10, []int{12}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var got []model.Question
			if err := tx.SelectContext(ctx, &got, newQuestionsForStudiedMaterialsQuery,
				42, "ja", pq.Array([]string{"N4"}), tt.category, pq.Array(tt.exclude), tt.limit, 0); err != nil {
				t.Fatal(err)
			}
			ids := make([]int, len(got))
			for i, q := range got {
				ids[i] = q.ID
			}
			if !reflect.DeepEqual(ids, tt.wantIDs) {
				t.Fatalf("question IDs = %v, want %v", ids, tt.wantIDs)
			}
		})
	}

	t.Run("new kanji recall respects remaining budget", func(t *testing.T) {
		var got []model.Question
		if err := tx.SelectContext(ctx, &got, newQuestionsForStudiedMaterialsQuery,
			42, "ja", pq.Array([]string{"N4"}), "vocabulary", pq.Array([]int{30}), 50, 1); err != nil {
			t.Fatal(err)
		}
		var recallIDs []int
		for _, q := range got {
			if q.Skill != nil && *q.Skill == model.SkillVocabKanjiRecall {
				recallIDs = append(recallIDs, q.ID)
			}
			if q.ID == 8 || q.ID == 9 || q.ID == 10 || q.ID == 30 {
				t.Fatalf("selected excluded, served, or wrong-scope question %d", q.ID)
			}
		}
		if !reflect.DeepEqual(recallIDs, []int{31}) {
			t.Fatalf("recall IDs = %v, want [31]", recallIDs)
		}
	})

	for _, statement := range []string{
		`INSERT INTO questions (id, material_id, proficiency_level) VALUES
			(101, 400, 'N4'), (102, 200, 'N5'), (103, 100, 'N4'),
			(104, 100, 'N4'), (105, 100, 'N4'), (106, 100, 'N3')`,
		`INSERT INTO questions (id, material_id, proficiency_level, item_type) VALUES
			(107, 100, 'N4', 'vocab_kanji_recall'), (108, 200, 'N5', 'vocab_kanji_recall')`,
		`INSERT INTO user_question_progress (user_id, question_id, next_review_at) VALUES
			(42, 101, NOW() - INTERVAL '1 day'), (42, 102, NOW() - INTERVAL '30 days'),
			(42, 103, NOW() - INTERVAL '2 days'), (42, 104, NOW() + INTERVAL '1 day'),
			(99, 105, NOW() - INTERVAL '30 days'), (42, 106, NOW() - INTERVAL '30 days'),
			(42, 107, NOW() - INTERVAL '1 day'), (42, 108, NOW() - INTERVAL '30 days')`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	for _, tt := range []struct {
		level   string
		wantIDs []int
	}{
		{"N4", []int{103, 101, 102}},
		{"N5", []int{102, 103, 101}},
	} {
		t.Run("due prioritizes current level "+tt.level, func(t *testing.T) {
			var got []model.Question
			if err := tx.SelectContext(
				ctx,
				&got,
				dueReviewsForStudiedMaterialsQuery,
				42,
				"ja",
				pq.Array([]string{"N5", "N4"}),
				10,
				0,
				tt.level,
				pq.Array([]model.QuestionCategory{}),
			); err != nil {
				t.Fatal(err)
			}
			ids := make([]int, len(got))
			for i, q := range got {
				ids[i] = q.ID
			}
			if !reflect.DeepEqual(ids, tt.wantIDs) {
				t.Fatalf("due IDs = %v, want %v", ids, tt.wantIDs)
			}
		})
	}
	t.Run("due recall cap preserves current level priority", func(t *testing.T) {
		var got []model.Question
		if err := tx.SelectContext(ctx, &got, dueReviewsForStudiedMaterialsQuery,
			42, "ja", pq.Array([]string{"N5", "N4"}), 10, 1, "N4", pq.Array([]model.QuestionCategory{})); err != nil {
			t.Fatal(err)
		}
		found := false
		for _, q := range got {
			if q.ID == 108 {
				t.Fatal("older adjacent-level recall displaced current-level recall")
			}
			found = found || q.ID == 107
		}
		if !found {
			t.Fatal("current-level recall missing")
		}
	})

	for _, statement := range []string{
		`INSERT INTO questions (id, material_id, category, audio_path) VALUES
			(201, 100, 'reading', NULL), (202, NULL, 'listening', 'ready.ogg'),
			(203, NULL, 'listening', NULL), (204, 400, 'reading', NULL)`,
		`INSERT INTO user_question_progress (user_id, question_id, next_review_at)
			SELECT 42, id, NOW() - INTERVAL '1 hour' FROM questions WHERE id BETWEEN 201 AND 204`,
		`INSERT INTO questions (id, material_id) SELECT id, 100 FROM generate_series(300, 329) AS id`,
		`INSERT INTO user_question_progress (user_id, question_id, next_review_at)
			SELECT 42, id, NOW() - INTERVAL '5 days' FROM questions WHERE id BETWEEN 300 AND 329`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	for _, tt := range []struct {
		category model.QuestionCategory
		wantID   int
	}{
		{model.CategoryReading, 201},
		{model.CategoryListening, 202},
	} {
		t.Run("reserved due category bypasses vocabulary backlog "+string(tt.category), func(t *testing.T) {
			var got []model.Question
			if err := tx.SelectContext(
				ctx,
				&got,
				dueReviewsForStudiedMaterialsQuery,
				42,
				"ja",
				pq.Array([]string{"N5", "N4"}),
				5,
				3,
				"N4",
				pq.Array([]model.QuestionCategory{tt.category}),
			); err != nil {
				t.Fatal(err)
			}
			if len(got) != 1 || got[0].ID != tt.wantID {
				t.Fatalf("reserved due selection = %v, want only %d", got, tt.wantID)
			}
		})
	}
	t.Run("reading cap applies before the bounded due pool", func(t *testing.T) {
		for _, statement := range []string{
			`INSERT INTO questions (id, material_id, category)
			 SELECT id, 100, 'reading' FROM generate_series(500, 529) AS id`,
			`INSERT INTO user_question_progress (user_id, question_id, next_review_at)
			 SELECT 42, id, NOW() - INTERVAL '10 days' FROM questions WHERE id BETWEEN 500 AND 529`,
		} {
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				t.Fatal(err)
			}
		}
		var got []model.Question
		if err := tx.SelectContext(ctx, &got, dueReviewsForStudiedMaterialsQuery,
			42, "ja", pq.Array([]string{"N5", "N4"}), 17, 3, "N4", pq.Array([]model.QuestionCategory{})); err != nil {
			t.Fatal(err)
		}
		reading := 0
		for _, q := range got {
			if q.Category == model.CategoryReading {
				reading++
			}
		}
		if len(got) != 17 || reading != 1 {
			t.Fatalf("due pool size/reading = %d/%d, want 17/1", len(got), reading)
		}
	})
}
