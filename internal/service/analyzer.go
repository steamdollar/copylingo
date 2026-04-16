package service

import (
	"context"

	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/repository"
)

// AnalyzerService provides learning analytics and recommendations.
type AnalyzerService struct {
	userRepo            *repository.UserRepository
	sessionQuestionRepo *repository.SessionQuestionRepository
}

func NewAnalyzerService(userRepo *repository.UserRepository, sessionQuestionRepo *repository.SessionQuestionRepository) *AnalyzerService {
	return &AnalyzerService{
		userRepo:            userRepo,
		sessionQuestionRepo: sessionQuestionRepo,
	}
}

// GetUserStats returns comprehensive learning statistics.
func (a *AnalyzerService) GetUserStats(ctx context.Context, userID int64) (*model.UserStats, error) {
	user, err := a.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	todayTotal, todayCorrect, err := a.sessionQuestionRepo.GetTodayStats(ctx)
	if err != nil {
		return nil, err
	}

	categoryAcc, err := a.sessionQuestionRepo.GetCategoryAccuracy(ctx)
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

// GetWeakAreas returns the user's weakest categories for targeted practice.
func (a *AnalyzerService) GetWeakAreas(ctx context.Context) ([]model.WeakArea, error) {
	categoryAcc, err := a.sessionQuestionRepo.GetCategoryAccuracy(ctx)
	if err != nil {
		return nil, err
	}

	var weakAreas []model.WeakArea
	for cat, acc := range categoryAcc {
		if acc < 60 { // Below 60% is considered weak
			weakAreas = append(weakAreas, model.WeakArea{
				Category: model.QuestionCategory(cat),
				Accuracy: acc,
			})
		}
	}

	return weakAreas, nil
}
