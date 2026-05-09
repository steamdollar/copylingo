package service

import (
	"context"
	"time"

	"github.com/lsj/copylingo/internal/model"
)

type questionQuerier interface {
	GetDueReviews(ctx context.Context, limit int) ([]model.Question, error)
	GetDueReviewCount(ctx context.Context) (int, error)
	UpdateSRS(ctx context.Context, q *model.Question) error
}

// srsScheduler는 GraderService와 SessionBuilderService가 SRSService에 의존할 때 쓰는 계약.
// *SRSService가 암묵적으로 만족한다.
type srsScheduler interface {
	GetDueReviews(ctx context.Context, limit int) ([]model.Question, error)
	GetDueCount(ctx context.Context) (int, error)
	ProcessAnswer(ctx context.Context, q *model.Question, isCorrect bool) error
}

// SRSService implements the SM-2 Spaced Repetition algorithm.
// SRS state is stored directly on the questions table.
type SRSService struct {
	questionRepo questionQuerier
}

func NewSRSService(questionRepo questionQuerier) *SRSService {
	return &SRSService{questionRepo: questionRepo}
}

// ProcessAnswer updates the SRS schedule on the question based on correctness.
func (s *SRSService) ProcessAnswer(ctx context.Context, question *model.Question, isCorrect bool) error {
	quality := 1 // wrong
	if isCorrect {
		quality = 4 // correct with some hesitation
	}

	s.updateSchedule(question, quality)

	return s.questionRepo.UpdateSRS(ctx, question)
}

// updateSchedule applies the SM-2 algorithm to update the question's SRS state.
func (s *SRSService) updateSchedule(q *model.Question, quality int) {
	now := time.Now()
	q.LastReviewedAt = &now

	if quality >= 3 { // Correct answer
		switch q.Repetitions {
		case 0:
			q.IntervalDays = 1
		case 1:
			q.IntervalDays = 6
		default:
			q.IntervalDays = int(float64(q.IntervalDays) * q.EaseFactor)
		}
		q.Repetitions++
	} else { // Wrong answer — reset
		q.Repetitions = 0
		q.IntervalDays = 1
	}

	// Update ease factor
	ef := q.EaseFactor + (0.1 - float64(5-quality)*(0.08+float64(5-quality)*0.02))
	if ef < 1.3 {
		ef = 1.3
	}
	q.EaseFactor = ef

	nextReview := now.AddDate(0, 0, q.IntervalDays)
	q.NextReviewAt = &nextReview
}

// GetDueReviews returns questions due for review.
func (s *SRSService) GetDueReviews(ctx context.Context, limit int) ([]model.Question, error) {
	return s.questionRepo.GetDueReviews(ctx, limit)
}

// GetDueCount returns the number of questions due for review.
func (s *SRSService) GetDueCount(ctx context.Context) (int, error) {
	return s.questionRepo.GetDueReviewCount(ctx)
}
