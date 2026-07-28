package repository

import (
	"strings"
	"testing"
	"time"

	"github.com/lsj/copylingo/internal/model"
)

func TestBuildQuestionProgressUpsertScopesByUserAndUsesAnswerDeltas(t *testing.T) {
	correct := true
	wrong := false
	nextReview := time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)
	items := []model.ActiveSessionQuestion{
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
	query, args, count := buildQuestionProgressUpsert([]model.ActiveSessionQuestion{{
		Question: model.Question{ID: 7},
		Progress: model.NewUserQuestionProgress(42, 7),
	}})
	if query != "" || args != nil || count != 0 {
		t.Fatalf("unexpected upsert for unanswered item: %q %#v %d", query, args, count)
	}
}
