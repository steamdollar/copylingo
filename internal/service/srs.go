package service

import (
	"context"
	"time"

	"github.com/lsj/copylingo/internal/model"
)

type questionQuerier interface {
	GetDueReviews(
		ctx context.Context,
		userID int64,
		language, level string,
		limit, kanjiRecallLimit int,
	) ([]model.Question, error)
	GetDueReviewCount(ctx context.Context, userID int64, language, level string) (int, error)
}

// srs: Spaced Repetition System
// srsScheduler는 GraderService와 SessionBuilderService가 SRSService에 의존할 때 쓰는 계약.
// *SRSService가 암묵적으로 만족한다.
type srsScheduler interface {
	GetDueReviews(
		ctx context.Context,
		userID int64,
		language, level string,
		limit, kanjiRecallLimit int,
	) ([]model.Question, error)
	GetDueCount(ctx context.Context, userID int64, language, level string) (int, error)
}

// SRSService implements the SM-2 Spaced Repetition algorithm for per-user progress.
type SRSService struct {
	questionRepo questionQuerier
}

func NewSRSService(questionRepo questionQuerier) *SRSService {
	return &SRSService{questionRepo: questionRepo}
}

// ScheduleAnswer applies SRS changes in memory without writing to the DB.
func (s *SRSService) ScheduleAnswer(progress *model.UserQuestionProgress, isCorrect bool) {
	quality := 1 // wrong
	if isCorrect {
		quality = 4 // correct with some hesitation
	}

	s.updateSchedule(progress, quality)
}

// updateSchedule applies the SM-2 algorithm to update the user's Question progress.
func (s *SRSService) updateSchedule(q *model.UserQuestionProgress, quality int) {
	now := time.Now()
	q.LastReviewedAt = &now

	if quality >= 3 { // Correct answer
		switch q.Repetitions {
		case 0:
			// 첫 복습 간격: 1일이면 하루 2회 push 환경에서 학습한 항목이
			// 다음날 곧장 due로 재등장한다. 3일로 늘려 매일 재출제를 끊는다.
			q.IntervalDays = 3
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
func (s *SRSService) GetDueReviews(
	ctx context.Context,
	userID int64,
	language, level string,
	limit, kanjiRecallLimit int,
) ([]model.Question, error) {
	return s.questionRepo.GetDueReviews(ctx, userID, language, level, limit, kanjiRecallLimit)
}

// GetDueCount returns the number of questions due for review.
func (s *SRSService) GetDueCount(
	ctx context.Context,
	userID int64,
	language, level string,
) (int, error) {
	return s.questionRepo.GetDueReviewCount(ctx, userID, language, level)
}
