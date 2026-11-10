package service

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/lsj/copylingo/internal/model"
)

type analyzerUserRepository interface {
	GetByID(
		ctx context.Context,
		id int64,
	) (*model.User, error)
}

type sessionStatsRepository interface {
	GetTodayStats(
		ctx context.Context,
		userID int64,
	) (int, int, error)
	GetCategoryAccuracy(
		ctx context.Context,
		userID int64,
	) (map[string]float64, error)
}

type materialProgressRepository interface {
	GetLevelProgress(
		ctx context.Context,
		userID int64,
		language string,
	) ([]model.LevelProgress, error)
}

type AnalyzerService struct {
	userRepository            analyzerUserRepository
	sessionQuestionRepository sessionStatsRepository
	materialRepository        materialProgressRepository
}

func NewAnalyzerService(
	userRepository analyzerUserRepository,
	sessionQuestionRepository sessionStatsRepository,
	materialRepository materialProgressRepository,
) *AnalyzerService {
	return &AnalyzerService{
		userRepository:            userRepository,
		sessionQuestionRepository: sessionQuestionRepository,
		materialRepository:        materialRepository,
	}
}

func (service *AnalyzerService) GetUserStats(
	ctx context.Context,
	userID int64,
) (*model.UserStats, error) {
	user, err := service.userRepository.GetByID(
		ctx,
		userID,
	)
	if err != nil {
		return nil, err
	}

	todayTotal, todayCorrect, err := service.sessionQuestionRepository.GetTodayStats(
		ctx,
		userID,
	)
	if err != nil {
		return nil, err
	}

	categoryAccuracy, err := service.sessionQuestionRepository.GetCategoryAccuracy(
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
		VocabularyAccuracy: categoryAccuracy[string(model.CategoryVocabulary)],
		GrammarAccuracy:    categoryAccuracy[string(model.CategoryGrammar)],
		KanjiAccuracy:      categoryAccuracy[string(model.CategoryKanji)],
		ReadingAccuracy:    categoryAccuracy[string(model.CategoryReading)],
		ListeningAccuracy:  categoryAccuracy[string(model.CategoryListening)],
	}

	if todayTotal > 0 {
		stats.OverallAccuracy = float64(todayCorrect) / float64(todayTotal) * 100
	}

	return stats, nil
}

// Japanese levels run NZ → N1; other languages keep the repository's lexical order.
func (service *AnalyzerService) GetLevelProgress(
	ctx context.Context,
	userID int64,
) ([]model.LevelProgress, error) {
	user, err := service.userRepository.GetByID(
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

	progress, err := service.materialRepository.GetLevelProgress(
		ctx,
		userID,
		user.Language,
	)
	if err != nil {
		return nil, err
	}

	// Unknown-order languages rank every level equally, so the stable sort keeps repository order.
	slices.SortStableFunc(
		progress,
		func(
			left,
			right model.LevelProgress,
		) int {
			return cmp.Compare(
				levelRank(
					user.Language,
					left.Level,
				),
				levelRank(
					user.Language,
					right.Level,
				),
			)
		},
	)
	return progress, nil
}
