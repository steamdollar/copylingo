package service

import (
	"context"

	"github.com/lsj/copylingo/internal/model"
)

// Read-only lookups for rendering and scheduling. They never change DB or
// Redis state beyond the Quiz working-set recovery done by QuizProgress.

func (service *SessionService) CountUnfinishedBatch(
	ctx context.Context,
	userIDs []int64,
) (map[int64]int, error) {
	return service.sessionRepository.CountUnfinishedBatch(
		ctx,
		userIDs,
	)
}

func (service *SessionService) OldestUnfinished(
	ctx context.Context,
	userID int64,
) (*model.Session, error) {
	return service.sessionRepository.GetOldestUnfinished(
		ctx,
		userID,
	)
}

func (service *SessionService) ListByStatus(
	ctx context.Context,
	userID int64,
	status model.SessionStatus,
) ([]model.Session, error) {
	return service.sessionRepository.GetSessionsByStatus(
		ctx,
		userID,
		status,
	)
}

func (service *SessionService) ListInProgressQuizzes(ctx context.Context) ([]model.Session, error) {
	return service.sessionRepository.ListInProgress(ctx)
}

func (service *SessionService) DueReviewCount(
	ctx context.Context,
	userID int64,
	language,
	level string,
) (int, error) {
	return service.questionRepository.GetDueReviewCount(
		ctx,
		userID,
		language,
		sessionLevelsFor(
			language,
			level,
		),
	)
}

// QuizProgress returns the Quiz working set, recovering it from DB when Redis has none.
func (service *SessionService) QuizProgress(
	ctx context.Context,
	sessionID int,
) (*model.QuizActiveSessionState, error) {
	return service.quizProgress.Get(
		ctx,
		sessionID,
	)
}
