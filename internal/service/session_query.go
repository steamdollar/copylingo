package service

import (
	"context"

	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
)

// Read-only lookups for rendering and scheduling. They never change DB or
// Redis state beyond the Quiz working-set recovery done by QuizProgress.

// CountUnfinishedBatch returns the number of pending/in-progress sessions of any mode per user.
func (s *SessionService) CountUnfinishedBatch(
	ctx context.Context,
	userIDs []int64,
) (map[int64]int, error) {
	return s.sessionRepo.CountUnfinishedBatch(
		ctx,
		userIDs,
	)
}

// OldestUnfinished returns the user's highest-priority unfinished session of any mode.
func (s *SessionService) OldestUnfinished(
	ctx context.Context,
	userID int64,
) (*model.Session, error) {
	return s.sessionRepo.GetOldestUnfinished(
		ctx,
		userID,
	)
}

// ListByStatus returns the user's sessions of any mode in the given status.
func (s *SessionService) ListByStatus(
	ctx context.Context,
	userID int64,
	status config.SessionStatus,
) ([]model.Session, error) {
	return s.sessionRepo.GetSessionsByStatus(
		ctx,
		userID,
		status,
	)
}

// ListInProgressQuizzes returns in-progress Quiz sessions of all users.
func (s *SessionService) ListInProgressQuizzes(ctx context.Context) ([]model.Session, error) {
	return s.sessionRepo.ListInProgress(ctx)
}

// DueReviewCount returns how many SRS items are due for the user's language/level scope.
func (s *SessionService) DueReviewCount(
	ctx context.Context,
	userID int64,
	language,
	level string,
) (int, error) {
	return s.srs.GetDueCount(
		ctx,
		userID,
		language,
		level,
	)
}

// QuizProgress returns the Quiz working set, recovering it from DB when Redis has none.
func (s *SessionService) QuizProgress(
	ctx context.Context,
	sessionID int,
) (*model.QuizActiveSessionState, error) {
	return s.quizProgress.Get(
		ctx,
		sessionID,
	)
}
