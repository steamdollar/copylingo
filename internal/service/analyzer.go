package service

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/lsj/copylingo/internal/model"
)

type analyzerUserRepo interface {
	GetByID(
		ctx context.Context,
		id int64,
	) (*model.User, error)
}

type sessionStatRepo interface {
	GetTodayStats(
		ctx context.Context,
		userID int64,
	) (int, int, error)
	GetCategoryAccuracy(
		ctx context.Context,
		userID int64,
	) (map[string]float64, error)
}

type materialProgressRepo interface {
	GetLevelProgress(
		ctx context.Context,
		userID int64,
		language string,
	) ([]model.LevelProgress, error)
}

// AnalyzerService provides learning analytics and recommendations.
type AnalyzerService struct {
	userRepo            analyzerUserRepo
	sessionQuestionRepo sessionStatRepo
	materialRepo        materialProgressRepo
}

func NewAnalyzerService(
	userRepo analyzerUserRepo,
	sessionQuestionRepo sessionStatRepo,
	materialRepo materialProgressRepo,
) *AnalyzerService {
	return &AnalyzerService{
		userRepo:            userRepo,
		sessionQuestionRepo: sessionQuestionRepo,
		materialRepo:        materialRepo,
	}
}

// GetUserStats returns comprehensive learning statistics.
func (a *AnalyzerService) GetUserStats(
	ctx context.Context,
	userID int64,
) (*model.UserStats, error) {
	user, err := a.userRepo.GetByID(
		ctx,
		userID,
	)
	if err != nil {
		return nil, err
	}

	todayTotal, todayCorrect, err := a.sessionQuestionRepo.GetTodayStats(
		ctx,
		userID,
	)
	if err != nil {
		return nil, err
	}

	categoryAcc, err := a.sessionQuestionRepo.GetCategoryAccuracy(
		ctx,
		userID,
	)
	if err != nil {
		return nil, err
	}

	stats := &model.UserStats{
		UserID:             userID,
		CurrentStreak:      user.StreakDays,
		TodayQuestions:     todayTotal,
		TodayCorrect:       todayCorrect,
		VocabularyAccuracy: categoryAcc[string(model.CategoryVocabulary)],
		GrammarAccuracy:    categoryAcc[string(model.CategoryGrammar)],
		KanjiAccuracy:      categoryAcc[string(model.CategoryKanji)],
		ReadingAccuracy:    categoryAcc[string(model.CategoryReading)],
		ListeningAccuracy:  categoryAcc[string(model.CategoryListening)],
	}

	// Overall accuracy
	if todayTotal > 0 {
		stats.OverallAccuracy = float64(todayCorrect) / float64(todayTotal) * 100
	}

	return stats, nil
}

// GetLevelProgress returns, for each level of the user's language, the share
// of materials studied at least once. Japanese levels run N5 → N1; other
// languages keep the repository's lexical order.
func (a *AnalyzerService) GetLevelProgress(
	ctx context.Context,
	userID int64,
) ([]model.LevelProgress, error) {
	user, err := a.userRepo.GetByID(
		ctx,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"AnalyzerService.GetLevelProgress user_id=%d: %w",
			userID,
			err,
		)
	}

	progress, err := a.materialRepo.GetLevelProgress(
		ctx,
		userID,
		user.Language,
	)
	if err != nil {
		return nil, err
	}

	if user.Language == "ja" {
		slices.SortStableFunc(
			progress,
			func(
				left,
				right model.LevelProgress,
			) int {
				return cmp.Compare(
					jlptLevelRank(left.Level),
					jlptLevelRank(right.Level),
				)
			},
		)
	}
	return progress, nil
}

// GetWeakAreas returns the user's weakest categories for targeted practice.
func (a *AnalyzerService) GetWeakAreas(
	ctx context.Context,
	userID int64,
) ([]model.WeakArea, error) {
	categoryAcc, err := a.sessionQuestionRepo.GetCategoryAccuracy(
		ctx,
		userID,
	)
	if err != nil {
		return nil, err
	}

	var weakAreas []model.WeakArea
	for cat, acc := range categoryAcc {
		if acc < 60 { // Below 60% is considered weak
			weakAreas = append(
				weakAreas,
				model.WeakArea{
					Category: model.QuestionCategory(cat),
					Accuracy: acc,
				},
			)
		}
	}

	return weakAreas, nil
}
