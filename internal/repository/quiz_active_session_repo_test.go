package repository

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/lsj/copylingo/internal/model"
)

func TestBuildQuestionProgressUpsertScopesByUserAndUsesAnswerDeltas(t *testing.T) {
	correct := true
	wrong := false
	nextReview := time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)
	items := []model.QuizActiveSessionQuestion{
		{
			SessionQuestion: model.SessionQuestion{QuestionID: 7, IsCorrect: &correct},
			Question:        model.Question{ID: 7},
			Progress: model.UserQuestionProgress{
				UserID:       42,
				QuestionID:   7,
				EaseFactor:   2.6,
				IntervalDays: 3,
				Repetitions:  1,
				NextReviewAt: &nextReview,
			},
		},
		{
			SessionQuestion: model.SessionQuestion{QuestionID: 8, IsCorrect: &wrong},
			Question:        model.Question{ID: 8},
			Progress:        model.NewUserQuestionProgress(42, 8),
		},
		{
			SessionQuestion: model.SessionQuestion{QuestionID: 9},
			Question:        model.Question{ID: 9},
			Progress:        model.NewUserQuestionProgress(99, 9),
		},
	}

	query, args, count := buildQuestionProgressUpsert(items)
	if count != 2 {
		t.Fatalf("count = %d, want 2 answered questions", count)
	}
	for _, want := range []string{
		"INSERT INTO user_question_progress",
		"ON CONFLICT (user_id, question_id) DO UPDATE",
		"user_question_progress.times_served + EXCLUDED.times_served",
		"updated_at = NOW()",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("query = %q, want %q", query, want)
		}
	}
	if len(args) != 18 {
		t.Fatalf("len(args) = %d, want 18", len(args))
	}
	if args[0] != int64(42) || args[1] != 7 || args[2] != 1 || args[3] != 1 {
		t.Fatalf("first row identity/deltas = %#v", args[:4])
	}
	if args[9] != int64(42) || args[10] != 8 || args[11] != 1 || args[12] != 0 {
		t.Fatalf("second row identity/deltas = %#v", args[9:13])
	}
}

func TestBuildQuestionProgressUpsertSkipsUnanswered(t *testing.T) {
	query, args, count := buildQuestionProgressUpsert([]model.QuizActiveSessionQuestion{{
		Question: model.Question{ID: 7},
		Progress: model.NewUserQuestionProgress(42, 7),
	}})
	if query != "" || args != nil || count != 0 {
		t.Fatalf("unexpected upsert for unanswered item: %q %#v %d", query, args, count)
	}
}

func TestQuizMaterialPreferenceCompletionPostgres(t *testing.T) {
	tx := preferenceTestTransaction(t)
	ctx := context.Background()
	for _, query := range []string{
		`INSERT INTO materials (id,material_key) SELECT id,'material-'||id FROM generate_series(1,10) id`,
		`INSERT INTO questions (id,material_id) SELECT id,id FROM materials`,
		`INSERT INTO questions (id,material_id) VALUES (22,2)`,
		`INSERT INTO user_material_preferences (user_id,material_id,review_mode,next_check_at,updated_at)
		 SELECT 42,id,'maintenance',NOW()-INTERVAL '1 day',NOW()-INTERVAL '2 days' FROM materials`,
		`INSERT INTO user_material_preferences (user_id,material_id,review_mode,next_check_at,updated_at)
		 VALUES (99,1,'maintenance',NOW()-INTERVAL '1 day',NOW()-INTERVAL '2 days')`,
		`UPDATE user_material_preferences SET review_mode='excluded',next_check_at=NULL WHERE user_id=42 AND material_id=3`,
		`UPDATE user_material_preferences SET next_check_at=NOW()-INTERVAL '30 minutes' WHERE user_id=42 AND material_id=4`,
		`UPDATE user_material_preferences SET updated_at=NOW()-INTERVAL '30 minutes' WHERE user_id=42 AND material_id=5`,
		`UPDATE user_material_preferences SET check_interval_days=CASE material_id WHEN 7 THEN 60 WHEN 8 THEN 120 ELSE 180 END WHERE user_id=42 AND material_id IN (7,8,9)`,
		`INSERT INTO sessions (id,user_id,created_at) VALUES (1,42,NOW()-INTERVAL '1 hour')`,
		`INSERT INTO session_questions (id,session_id,question_id) SELECT id,1,id FROM questions`,
	} {
		mustPreferenceExec(t, tx, query)
	}
	correct, wrong := true, false
	state := &model.QuizActiveSessionState{Session: model.Session{ID: 1, UserID: 42}}
	for _, id := range []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 22} {
		answer := &correct
		if id == 3 || id == 4 || id == 22 {
			answer = &wrong
		}
		if id == 6 {
			answer = nil
		}
		state.Items = append(state.Items, model.QuizActiveSessionQuestion{
			SessionQuestion: model.SessionQuestion{ID: id, QuestionID: id, IsCorrect: answer},
			Question:        model.Question{ID: id}, Progress: model.NewUserQuestionProgress(42, id),
		})
	}
	flush := func() bool {
		t.Helper()
		flushed, err := markSessionCompleted(ctx, tx, state)
		if err != nil {
			t.Fatal(err)
		}
		if flushed {
			if err := flushSessionQuestions(ctx, tx, state.Items); err != nil {
				t.Fatal(err)
			}
			if err := flushQuestionProgress(ctx, tx, state.Items); err != nil {
				t.Fatal(err)
			}
			if err := flushQuizMaterialPreferences(ctx, tx, 1); err != nil {
				t.Fatal(err)
			}
		}
		return flushed
	}
	if !flush() {
		t.Fatal("first completion did not flush")
	}
	repo := &MaterialPreferenceRepository{db: tx}
	for _, tt := range []struct {
		id   int
		mode model.MaterialReviewMode
		days int
	}{
		{1, model.MaterialReviewMaintenance, 60}, {2, model.MaterialReviewNormal, 30},
		{3, model.MaterialReviewExcluded, 30}, {4, model.MaterialReviewMaintenance, 30},
		{5, model.MaterialReviewMaintenance, 30}, {6, model.MaterialReviewMaintenance, 30},
		{7, model.MaterialReviewMaintenance, 120}, {8, model.MaterialReviewMaintenance, 180},
		{9, model.MaterialReviewMaintenance, 180}, {10, model.MaterialReviewMaintenance, 60},
	} {
		got, err := repo.Get(ctx, 42, tt.id)
		if err != nil || got.ReviewMode != tt.mode || got.CheckIntervalDays != tt.days {
			t.Fatalf("material %d preference=%+v, err=%v", tt.id, got, err)
		}
		if tt.id == 1 || tt.id >= 7 {
			var delta float64
			if err := tx.GetContext(
				ctx,
				&delta,
				`SELECT EXTRACT(EPOCH FROM p.next_check_at-s.completed_at)/86400 FROM user_material_preferences p CROSS JOIN sessions s WHERE p.user_id=42 AND p.material_id=$1 AND s.id=1`,
				tt.id,
			); err != nil {
				t.Fatal(err)
			}
			if delta != float64(tt.days) {
				t.Fatalf("material %d completion-based interval=%v, want %d", tt.id, delta, tt.days)
			}
		}
	}
	other, err := repo.Get(ctx, 99, 1)
	if err != nil || other.CheckIntervalDays != 30 {
		t.Fatalf("other user changed: %+v, %v", other, err)
	}
	before, err := repo.Get(ctx, 42, 1)
	if err != nil {
		t.Fatal(err)
	}
	if flush() {
		t.Fatal("repeated completion flushed again")
	}
	// The moved gate itself also prevents replay even if the helper is invoked.
	if err := flushQuizMaterialPreferences(ctx, tx, 1); err != nil {
		t.Fatal(err)
	}
	after, err := repo.Get(ctx, 42, 1)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("replay changed preference: %+v -> %+v, %v", before, after, err)
	}
	var served int
	if err := tx.GetContext(
		ctx,
		&served,
		`SELECT times_served FROM user_question_progress WHERE user_id=42 AND question_id=1`,
	); err != nil ||
		served != 1 {
		t.Fatalf("replay applied progress twice: %d, %v", served, err)
	}
}
