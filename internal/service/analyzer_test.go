package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

type mockAnalyzerUserRepository struct {
	getByIDFunc func(
		ctx context.Context,
		id int64,
	) (*model.User, error)
}

func (mock *mockAnalyzerUserRepository) GetByID(
	ctx context.Context,
	id int64,
) (*model.User, error) {
	return mock.getByIDFunc(
		ctx,
		id,
	)
}

type mockSessionStatsRepository struct {
	getTodayStatsFunc func(
		ctx context.Context,
		userID int64,
	) (int, int, error)
	getCategoryAccuracyFunc func(
		ctx context.Context,
		userID int64,
	) (map[string]float64, error)
}

func (mock *mockSessionStatsRepository) GetTodayStats(
	ctx context.Context,
	userID int64,
) (int, int, error) {
	return mock.getTodayStatsFunc(
		ctx,
		userID,
	)
}
func (mock *mockSessionStatsRepository) GetCategoryAccuracy(
	ctx context.Context,
	userID int64,
) (map[string]float64, error) {
	return mock.getCategoryAccuracyFunc(
		ctx,
		userID,
	)
}

type mockMaterialProgressRepository struct {
	getLevelProgressFunc func(
		ctx context.Context,
		userID int64,
		language string,
	) ([]model.LevelProgress, error)
}

func (mock *mockMaterialProgressRepository) GetLevelProgress(
	ctx context.Context,
	userID int64,
	language string,
) ([]model.LevelProgress, error) {
	return mock.getLevelProgressFunc(
		ctx,
		userID,
		language,
	)
}

func TestGetUserStats_MapsFieldsCorrectly(t *testing.T) {
	ctx := context.Background()
	userID := int64(123)

	userRepositoryMock := &mockAnalyzerUserRepository{
		getByIDFunc: func(
			ctx context.Context,
			id int64,
		) (*model.User, error) {
			return &model.User{ID: id, StreakDays: 5}, nil
		},
	}

	statsRepositoryMock := &mockSessionStatsRepository{
		getTodayStatsFunc: func(
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
		getCategoryAccuracyFunc: func(
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
		userRepositoryMock,
		statsRepositoryMock,
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

	userRepositoryMock := &mockAnalyzerUserRepository{
		getByIDFunc: func(
			ctx context.Context,
			id int64,
		) (*model.User, error) {
			return &model.User{ID: id}, nil
		},
	}

	statsRepositoryMock := &mockSessionStatsRepository{
		getTodayStatsFunc: func(
			ctx context.Context,
			userID int64,
		) (int, int, error) {
			return 0, 0, nil
		},
		getCategoryAccuracyFunc: func(
			ctx context.Context,
			userID int64,
		) (map[string]float64, error) {
			return map[string]float64{}, nil
		},
	}

	analyzer := NewAnalyzerService(
		userRepositoryMock,
		statsRepositoryMock,
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
	userRepositoryMock := &mockAnalyzerUserRepository{
		getByIDFunc: func(
			ctx context.Context,
			id int64,
		) (*model.User, error) {
			return nil, errors.New("user not found")
		},
	}

	analyzer := NewAnalyzerService(
		userRepositoryMock,
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

func TestGetLevelProgress_OrdersJapaneseLevelsFromNZ(t *testing.T) {
	userRepositoryMock := &mockAnalyzerUserRepository{
		getByIDFunc: func(
			ctx context.Context,
			id int64,
		) (*model.User, error) {
			return &model.User{ID: id, Language: "ja"}, nil
		},
	}
	materialProgressRepositoryMock := &mockMaterialProgressRepository{
		getLevelProgressFunc: func(
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
			// Repository returns lexical order: N1 < N4 < N5 < NZ.
			return []model.LevelProgress{
				{Level: "N1", TotalMaterials: 10},
				{Level: "N4", TotalMaterials: 20, StudiedMaterials: 5},
				{Level: "N5", TotalMaterials: 30, StudiedMaterials: 30},
				{Level: "NZ", TotalMaterials: 40, StudiedMaterials: 40},
			}, nil
		},
	}

	progress, err := NewAnalyzerService(
		userRepositoryMock,
		nil,
		materialProgressRepositoryMock,
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
	) != "NZ,N5,N4,N1" {
		t.Fatalf(
			"levels = %v, want [NZ N5 N4 N1]",
			gotLevels,
		)
	}
}
