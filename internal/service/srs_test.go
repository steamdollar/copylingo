package service

import (
	"context"
	"errors"
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

type mockQuestionQuerier struct {
	getDueReviewsFn     func(ctx context.Context, limit int) ([]model.Question, error)
	getDueReviewCountFn func(ctx context.Context) (int, error)
	updateSRSFn         func(ctx context.Context, q *model.Question) error
}

func (m *mockQuestionQuerier) GetDueReviews(ctx context.Context, limit int) ([]model.Question, error) {
	return m.getDueReviewsFn(ctx, limit)
}

func (m *mockQuestionQuerier) GetDueReviewCount(ctx context.Context) (int, error) {
	return m.getDueReviewCountFn(ctx)
}

func (m *mockQuestionQuerier) UpdateSRS(ctx context.Context, q *model.Question) error {
	return m.updateSRSFn(ctx, q)
}

func TestProcessAnswer(t *testing.T) {
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
			initialRep:    0,
			initialInt:    0,
			initialEase:   2.5,
			isCorrect:     true,
			expectRep:     1,
			expectInt:     1,
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
			expectInt:     15, // 6 * 2.5
			expectEaseGte: 1.3,
		},
		{
			name:          "WrongResetsRepetitions",
			initialRep:    3,
			initialInt:    20,
			initialEase:   2.5,
			isCorrect:     false,
			expectRep:     0,
			expectInt:     1,
			expectEaseGte: 1.3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedQuestion *model.Question
			mockRepo := &mockQuestionQuerier{
				updateSRSFn: func(ctx context.Context, q *model.Question) error {
					capturedQuestion = q
					return nil
				},
			}

			srs := NewSRSService(mockRepo)
			q := &model.Question{
				Repetitions:  tt.initialRep,
				IntervalDays: tt.initialInt,
				EaseFactor:   tt.initialEase,
			}

			err := srs.ProcessAnswer(context.Background(), q, tt.isCorrect)
			if err != nil {
				t.Fatalf("ProcessAnswer failed: %v", err)
			}

			if q.Repetitions != tt.expectRep {
				t.Errorf("expected Repetitions %d, got %d", tt.expectRep, q.Repetitions)
			}
			if q.IntervalDays != tt.expectInt {
				t.Errorf("expected IntervalDays %d, got %d", tt.expectInt, q.IntervalDays)
			}
			if q.EaseFactor < tt.expectEaseGte {
				t.Errorf("expected EaseFactor >= %f, got %f", tt.expectEaseGte, q.EaseFactor)
			}
			if q.NextReviewAt == nil {
				t.Error("expected NextReviewAt to be set, got nil")
			}
			if q.LastReviewedAt == nil {
				t.Error("expected LastReviewedAt to be set, got nil")
			}
			if capturedQuestion != q {
				t.Error("UpdateSRS was not called with the correct question")
			}
		})
	}
}

func TestEaseFactorFloorAt1_3(t *testing.T) {
	mockRepo := &mockQuestionQuerier{
		updateSRSFn: func(ctx context.Context, q *model.Question) error {
			return nil
		},
	}
	srs := NewSRSService(mockRepo)

	q := &model.Question{
		Repetitions:  1,
		IntervalDays: 1,
		EaseFactor:   1.3,
	}

	// Multiple wrong answers to push EF down
	for i := 0; i < 5; i++ {
		_ = srs.ProcessAnswer(context.Background(), q, false)
		if q.EaseFactor < 1.3 {
			t.Errorf("EaseFactor dropped below 1.3: %f", q.EaseFactor)
		}
	}
}

func TestProcessAnswer_UpdateSRSFails(t *testing.T) {
	expectedErr := errors.New("update srs failed")
	mockRepo := &mockQuestionQuerier{
		updateSRSFn: func(ctx context.Context, q *model.Question) error {
			return expectedErr
		},
	}
	srs := NewSRSService(mockRepo)

	q := &model.Question{
		Repetitions:  0,
		IntervalDays: 0,
		EaseFactor:   2.5,
	}

	err := srs.ProcessAnswer(context.Background(), q, true)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected UpdateSRS error %v, got %v", expectedErr, err)
	}
	if q.NextReviewAt == nil {
		t.Error("expected schedule to be updated before repository failure")
	}
}
