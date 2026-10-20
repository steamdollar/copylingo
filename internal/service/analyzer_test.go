package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

type mockAnalyzerUserRepo struct {
	getByIDFn func(
		ctx context.Context,
		id int64,
	) (*model.User, error)
}

func (m *mockAnalyzerUserRepo) GetByID(
	ctx context.Context,
	id int64,
) (*model.User, error) {
	return m.getByIDFn(
		ctx,
		id,
	)
}

type mockSessionStatRepo struct {
	getTodayStatsFn func(
		ctx context.Context,
		userID int64,
	) (int, int, error)
	getCategoryAccuracyFn func(
		ctx context.Context,
		userID int64,
	) (map[string]float64, error)
}

func (m *mockSessionStatRepo) GetTodayStats(
	ctx context.Context,
	userID int64,
) (int, int, error) {
	return m.getTodayStatsFn(
		ctx,
		userID,
	)
}
func (m *mockSessionStatRepo) GetCategoryAccuracy(
	ctx context.Context,
	userID int64,
) (map[string]float64, error) {
	return m.getCategoryAccuracyFn(
		ctx,
		userID,
	)
}

type mockMaterialProgressRepo struct {
	getLevelProgressFn func(
		ctx context.Context,
		userID int64,
		language string,
	) ([]model.LevelProgress, error)
}

func (m *mockMaterialProgressRepo) GetLevelProgress(
	ctx context.Context,
	userID int64,
	language string,
) ([]model.LevelProgress, error) {
	return m.getLevelProgressFn(
		ctx,
		userID,
		language,
	)
}

func TestGetUserStats_MapsFieldsCorrectly(t *testing.T) {
	ctx := context.Background()
	userID := int64(123)

	uRepo := &mockAnalyzerUserRepo{
		getByIDFn: func(
			ctx context.Context,
			id int64,
		) (*model.User, error) {
			return &model.User{ID: id, StreakDays: 5}, nil
		},
	}

	sRepo := &mockSessionStatRepo{
		getTodayStatsFn: func(
			ctx context.Context,
			gotUserID int64,
		) (int, int, error) {
			if gotUserID != userID {
				t.Fatalf(
					"userID = %d, want %d",
					gotUserID,
					userID,
				)
			}
			return 10, 7, nil
		},
		getCategoryAccuracyFn: func(
			ctx context.Context,
			gotUserID int64,
		) (map[string]float64, error) {
			if gotUserID != userID {
				t.Fatalf(
					"userID = %d, want %d",
					gotUserID,
					userID,
				)
			}
			return map[string]float64{
				string(model.CategoryVocabulary): 80.0,
			}, nil
		},
	}

	analyzer := NewAnalyzerService(
		uRepo,
		sRepo,
		nil,
	)
	stats, err := analyzer.GetUserStats(
		ctx,
		userID,
	)

	if err != nil {
		t.Fatalf(
			"GetUserStats failed: %v",
			err,
		)
	}
	if stats.CurrentStreak != 5 {
		t.Errorf(
			"expected streak 5, got %d",
			stats.CurrentStreak,
		)
	}
	if stats.OverallAccuracy != 70.0 {
		t.Errorf(
			"expected overall accuracy 70.0, got %f",
			stats.OverallAccuracy,
		)
	}
	if stats.VocabularyAccuracy != 80.0 {
		t.Errorf(
			"expected vocab accuracy 80.0, got %f",
			stats.VocabularyAccuracy,
		)
	}
}

func TestGetUserStats_ZeroDivision(t *testing.T) {
	ctx := context.Background()
	userID := int64(123)

	uRepo := &mockAnalyzerUserRepo{
		getByIDFn: func(
			ctx context.Context,
			id int64,
		) (*model.User, error) {
			return &model.User{ID: id}, nil
		},
	}

	sRepo := &mockSessionStatRepo{
		getTodayStatsFn: func(
			ctx context.Context,
			userID int64,
		) (int, int, error) {
			return 0, 0, nil
		},
		getCategoryAccuracyFn: func(
			ctx context.Context,
			userID int64,
		) (map[string]float64, error) {
			return map[string]float64{}, nil
		},
	}

	analyzer := NewAnalyzerService(
		uRepo,
		sRepo,
		nil,
	)
	stats, err := analyzer.GetUserStats(
		ctx,
		userID,
	)

	if err != nil {
		t.Fatalf(
			"GetUserStats failed: %v",
			err,
		)
	}
	if stats.OverallAccuracy != 0 {
		t.Errorf(
			"expected overall accuracy 0, got %f",
			stats.OverallAccuracy,
		)
	}
}

func TestGetUserStats_UserNotFound(t *testing.T) {
	ctx := context.Background()
	uRepo := &mockAnalyzerUserRepo{
		getByIDFn: func(
			ctx context.Context,
			id int64,
		) (*model.User, error) {
			return nil, errors.New("user not found")
		},
	}

	analyzer := NewAnalyzerService(
		uRepo,
		nil,
		nil,
	)
	_, err := analyzer.GetUserStats(
		ctx,
		123,
	)

	if err == nil {
		t.Error("expected error when user not found")
	}
}

func TestGetLevelProgress_OrdersJapaneseLevelsFromN5(t *testing.T) {
	uRepo := &mockAnalyzerUserRepo{
		getByIDFn: func(
			ctx context.Context,
			id int64,
		) (*model.User, error) {
			return &model.User{ID: id, Language: "ja"}, nil
		},
	}
	mRepo := &mockMaterialProgressRepo{
		getLevelProgressFn: func(
			ctx context.Context,
			userID int64,
			language string,
		) ([]model.LevelProgress, error) {
			if userID != 123 || language != "ja" {
				t.Fatalf(
					"GetLevelProgress(user=%d, language=%q), want (123, \"ja\")",
					userID,
					language,
				)
			}
			// Repository returns lexical order: N1 < N4 < N5.
			return []model.LevelProgress{
				{Level: "N1", TotalMaterials: 10},
				{Level: "N4", TotalMaterials: 20, StudiedMaterials: 5},
				{Level: "N5", TotalMaterials: 30, StudiedMaterials: 30},
			}, nil
		},
	}

	progress, err := NewAnalyzerService(
		uRepo,
		nil,
		mRepo,
	).GetLevelProgress(
		context.Background(),
		123,
	)
	if err != nil {
		t.Fatalf(
			"GetLevelProgress failed: %v",
			err,
		)
	}
	var gotLevels []string
	for _, level := range progress {
		gotLevels = append(
			gotLevels,
			level.Level,
		)
	}
	if strings.Join(
		gotLevels,
		",",
	) != "N5,N4,N1" {
		t.Fatalf(
			"levels = %v, want [N5 N4 N1]",
			gotLevels,
		)
	}
}
