package service

import (
	"context"
	"errors"
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

type mockQuestionQuerier struct {
	getDueReviewsFn func(
		ctx context.Context,
		userID int64,
		language, level string,
		limit, kanjiRecallLimit int,
	) ([]model.Question, error)
	getDueReviewCountFn func(ctx context.Context, userID int64, language, level string) (int, error)
}

func (m *mockQuestionQuerier) GetDueReviews(
	ctx context.Context,
	userID int64,
	language, level string,
	limit, kanjiRecallLimit int,
) ([]model.Question, error) {
	return m.getDueReviewsFn(ctx, userID, language, level, limit, kanjiRecallLimit)
}

func (m *mockQuestionQuerier) GetDueReviewCount(
	ctx context.Context,
	userID int64,
	language, level string,
) (int, error) {
	return m.getDueReviewCountFn(ctx, userID, language, level)
}

func TestScheduleAnswer(t *testing.T) {
	tests := []struct {
		name          string
		initialRep    int
		initialInt    int
		initialEase   float64
		isCorrect     bool
		expectRep     int
		expectInt     int
		expectEaseGte float64
	}{
		{
			name:          "CorrectFirstRepetition",
			initialEase:   2.5,
			isCorrect:     true,
			expectRep:     1,
			expectInt:     3,
			expectEaseGte: 1.3,
		},
		{
			name:          "CorrectSecondRepetition",
			initialRep:    1,
			initialInt:    1,
			initialEase:   2.5,
			isCorrect:     true,
			expectRep:     2,
			expectInt:     6,
			expectEaseGte: 1.3,
		},
		{
			name:          "CorrectSubsequentUsesFactor",
			initialRep:    2,
			initialInt:    6,
			initialEase:   2.5,
			isCorrect:     true,
			expectRep:     3,
			expectInt:     15,
			expectEaseGte: 1.3,
		},
		{
			name:          "WrongResetsRepetitions",
			initialRep:    3,
			initialInt:    20,
			initialEase:   2.5,
			expectRep:     0,
			expectInt:     1,
			expectEaseGte: 1.3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			progress := &model.UserQuestionProgress{
				Repetitions:  tt.initialRep,
				IntervalDays: tt.initialInt,
				EaseFactor:   tt.initialEase,
			}
			NewSRSService(nil).ScheduleAnswer(progress, tt.isCorrect)

			if progress.Repetitions != tt.expectRep {
				t.Errorf("Repetitions = %d, want %d", progress.Repetitions, tt.expectRep)
			}
			if progress.IntervalDays != tt.expectInt {
				t.Errorf("IntervalDays = %d, want %d", progress.IntervalDays, tt.expectInt)
			}
			if progress.EaseFactor < tt.expectEaseGte {
				t.Errorf("EaseFactor = %f, want >= %f", progress.EaseFactor, tt.expectEaseGte)
			}
			if progress.NextReviewAt == nil || progress.LastReviewedAt == nil {
				t.Fatal("expected review timestamps")
			}
		})
	}
}

func TestEaseFactorFloorAt1_3(t *testing.T) {
	srs := NewSRSService(nil)
	progress := &model.UserQuestionProgress{Repetitions: 1, IntervalDays: 1, EaseFactor: 1.3}
	for range 5 {
		srs.ScheduleAnswer(progress, false)
		if progress.EaseFactor < 1.3 {
			t.Fatalf("EaseFactor dropped below 1.3: %f", progress.EaseFactor)
		}
	}
}

func TestSRSService_GetDueReviewsForwardsUserScope(t *testing.T) {
	want := []model.Question{{ID: 1}, {ID: 2}}
	repo := &mockQuestionQuerier{
		getDueReviewsFn: func(
			_ context.Context,
			userID int64,
			language, level string,
			limit, kanjiRecallLimit int,
		) ([]model.Question, error) {
			if userID != 42 || language != "ja" || level != "N5" || limit != 10 || kanjiRecallLimit != 3 {
				t.Fatalf("unexpected scope: %d %s %s %d %d", userID, language, level, limit, kanjiRecallLimit)
			}
			return want, nil
		},
	}

	got, err := NewSRSService(repo).GetDueReviews(context.Background(), 42, "ja", "N5", 10, 3)
	if err != nil || len(got) != len(want) {
		t.Fatalf("GetDueReviews() = %v, %v", got, err)
	}
}

func TestSRSService_GetDueReviewsPropagatesError(t *testing.T) {
	expectedErr := errors.New("query failed")
	repo := &mockQuestionQuerier{
		getDueReviewsFn: func(
			context.Context,
			int64,
			string, string,
			int, int,
		) ([]model.Question, error) {
			return nil, expectedErr
		},
	}
	_, err := NewSRSService(repo).GetDueReviews(context.Background(), 1, "ja", "N5", 1, 0)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("GetDueReviews() error = %v, want %v", err, expectedErr)
	}
}

func TestSRSService_GetDueCountForwardsUserScope(t *testing.T) {
	repo := &mockQuestionQuerier{
		getDueReviewCountFn: func(_ context.Context, userID int64, language, level string) (int, error) {
			if userID != 42 || language != "ja" || level != "N5" {
				t.Fatalf("unexpected scope: %d %s %s", userID, language, level)
			}
			return 7, nil
		},
	}
	got, err := NewSRSService(repo).GetDueCount(context.Background(), 42, "ja", "N5")
	if err != nil || got != 7 {
		t.Fatalf("GetDueCount() = %d, %v", got, err)
	}
}
