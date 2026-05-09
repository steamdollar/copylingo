package service

import (
	"context"
	"errors"
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

type mockAnalyzerUserRepo struct {
	getByIDFn func(ctx context.Context, id int64) (*model.User, error)
}

func (m *mockAnalyzerUserRepo) GetByID(ctx context.Context, id int64) (*model.User, error) {
	return m.getByIDFn(ctx, id)
}

type mockSessionStatRepo struct {
	getTodayStatsFn       func(ctx context.Context) (int, int, error)
	getCategoryAccuracyFn func(ctx context.Context) (map[string]float64, error)
}

func (m *mockSessionStatRepo) GetTodayStats(ctx context.Context) (int, int, error) {
	return m.getTodayStatsFn(ctx)
}
func (m *mockSessionStatRepo) GetCategoryAccuracy(ctx context.Context) (map[string]float64, error) {
	return m.getCategoryAccuracyFn(ctx)
}

func TestGetUserStats_MapsFieldsCorrectly(t *testing.T) {
	ctx := context.Background()
	userID := int64(123)

	uRepo := &mockAnalyzerUserRepo{
		getByIDFn: func(ctx context.Context, id int64) (*model.User, error) {
			return &model.User{ID: id, StreakDays: 5}, nil
		},
	}

	sRepo := &mockSessionStatRepo{
		getTodayStatsFn: func(ctx context.Context) (int, int, error) {
			return 10, 7, nil
		},
		getCategoryAccuracyFn: func(ctx context.Context) (map[string]float64, error) {
			return map[string]float64{
				string(model.CategoryVocabulary): 80.0,
			}, nil
		},
	}

	analyzer := NewAnalyzerService(uRepo, sRepo)
	stats, err := analyzer.GetUserStats(ctx, userID)

	if err != nil {
		t.Fatalf("GetUserStats failed: %v", err)
	}
	if stats.CurrentStreak != 5 {
		t.Errorf("expected streak 5, got %d", stats.CurrentStreak)
	}
	if stats.OverallAccuracy != 70.0 {
		t.Errorf("expected overall accuracy 70.0, got %f", stats.OverallAccuracy)
	}
	if stats.VocabularyAccuracy != 80.0 {
		t.Errorf("expected vocab accuracy 80.0, got %f", stats.VocabularyAccuracy)
	}
}

func TestGetUserStats_ZeroDivision(t *testing.T) {
	ctx := context.Background()
	userID := int64(123)

	uRepo := &mockAnalyzerUserRepo{
		getByIDFn: func(ctx context.Context, id int64) (*model.User, error) {
			return &model.User{ID: id}, nil
		},
	}

	sRepo := &mockSessionStatRepo{
		getTodayStatsFn: func(ctx context.Context) (int, int, error) {
			return 0, 0, nil
		},
		getCategoryAccuracyFn: func(ctx context.Context) (map[string]float64, error) {
			return map[string]float64{}, nil
		},
	}

	analyzer := NewAnalyzerService(uRepo, sRepo)
	stats, err := analyzer.GetUserStats(ctx, userID)

	if err != nil {
		t.Fatalf("GetUserStats failed: %v", err)
	}
	if stats.OverallAccuracy != 0 {
		t.Errorf("expected overall accuracy 0, got %f", stats.OverallAccuracy)
	}
}

func TestGetWeakAreas_FiltersBelow60(t *testing.T) {
	ctx := context.Background()

	sRepo := &mockSessionStatRepo{
		getCategoryAccuracyFn: func(ctx context.Context) (map[string]float64, error) {
			return map[string]float64{
				"vocabulary": 59.9,
				"grammar":    60.0,
				"kanji":      75.0,
			}, nil
		},
	}

	analyzer := NewAnalyzerService(nil, sRepo)
	weakAreas, err := analyzer.GetWeakAreas(ctx)

	if err != nil {
		t.Fatalf("GetWeakAreas failed: %v", err)
	}
	if len(weakAreas) != 1 {
		t.Errorf("expected 1 weak area, got %d", len(weakAreas))
	}
	if weakAreas[0].Category != "vocabulary" {
		t.Errorf("expected vocabulary to be weak, got %s", weakAreas[0].Category)
	}
}

func TestGetUserStats_UserNotFound(t *testing.T) {
	ctx := context.Background()
	uRepo := &mockAnalyzerUserRepo{
		getByIDFn: func(ctx context.Context, id int64) (*model.User, error) {
			return nil, errors.New("user not found")
		},
	}

	analyzer := NewAnalyzerService(uRepo, nil)
	_, err := analyzer.GetUserStats(ctx, 123)

	if err == nil {
		t.Error("expected error when user not found")
	}
}
