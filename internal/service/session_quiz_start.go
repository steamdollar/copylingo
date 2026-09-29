package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/lsj/copylingo/internal/model"
)

// maxReviewQuizQuestions caps an on-demand review Quiz regardless of the due backlog.
const maxReviewQuizQuestions = 15

var (
	// ErrNoDueReviews means an on-demand review Quiz has nothing to review.
	ErrNoDueReviews = errors.New("no due review questions")
	// ErrQuizStatePrepareFailed means the Quiz was marked started in DB but
	// its Redis working set could not be reloaded to reflect that.
	ErrQuizStatePrepareFailed = errors.New("quiz working set prepare failed")
)

// BuildMorningQuiz creates the morning Quiz session; nil means no questions were available.
func (s *SessionService) BuildMorningQuiz(
	ctx context.Context,
	user model.User,
) (*model.Session, error) {
	return s.selection.BuildMorningSession(
		ctx,
		user.ID,
		user.Language,
		user.ProficiencyLevel,
	)
}

// BuildEveningQuiz creates the evening Quiz session; nil means no questions were available.
func (s *SessionService) BuildEveningQuiz(
	ctx context.Context,
	user model.User,
) (*model.Session, error) {
	return s.selection.BuildEveningSession(
		ctx,
		user.ID,
		user.Language,
		user.ProficiencyLevel,
	)
}

// BuildReviewQuiz creates an on-demand review Quiz of at most
// maxReviewQuizQuestions due items. A due-count lookup failure is treated as
// zero due items (existing policy), so it surfaces as ErrNoDueReviews.
func (s *SessionService) BuildReviewQuiz(
	ctx context.Context,
	user model.User,
) (*model.Session, error) {
	dueCount, err := s.srs.GetDueCount(
		ctx,
		user.ID,
		user.Language,
		user.ProficiencyLevel,
	)
	if err != nil {
		slog.WarnContext(
			ctx,
			"Due review count failed; treating as no due reviews",
			"event",
			"session.review.due_count_failed",
			"source",
			"service.session",
			"user_id",
			user.ID,
			"error",
			err,
		)
		dueCount = 0
	}
	if dueCount == 0 {
		return nil, ErrNoDueReviews
	}
	return s.selection.BuildReviewSession(
		ctx,
		user.ID,
		user.Language,
		user.ProficiencyLevel,
		min(
			dueCount,
			maxReviewQuizQuestions,
		),
	)
}

// StartQuiz marks the Quiz in_progress and returns its working set.
//
// Redis is the source of truth while a Quiz is in progress: an in-progress
// working set is never replaced from DB, so a repeated start button cannot
// overwrite recorded answers. Only a pending session (no progress yet) is
// reloaded after the DB transition so Redis reflects the new status.
// A zero session owner skips the owner check (legacy rows).
func (s *SessionService) StartQuiz(
	ctx context.Context,
	sessionID int,
	userID int64,
) (*model.QuizActiveSessionState, error) {
	state, err := s.quizProgress.Get(
		ctx,
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"load quiz state before start session_id=%d: %w",
			sessionID,
			err,
		)
	}
	if state.Session.UserID != 0 && state.Session.UserID != userID {
		return nil, fmt.Errorf(
			"%w session_id=%d user_id=%d",
			ErrQuizActiveSessionUserMismatch,
			sessionID,
			userID,
		)
	}
	wasPending := state.Session.Status == model.SessionPending

	if err := s.sessionRepo.Start(
		ctx,
		sessionID,
	); err != nil {
		return nil, fmt.Errorf(
			"mark quiz session started session_id=%d: %w",
			sessionID,
			err,
		)
	}
	if !wasPending {
		return state, nil
	}
	state, err = s.quizProgress.CreateFromDB(
		ctx,
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"%w session_id=%d: %w",
			ErrQuizStatePrepareFailed,
			sessionID,
			err,
		)
	}
	return state, nil
}

// ShowQuizQuestion moves the working-set cursor to questionIdx and returns the
// state to render, with one Redis read. An index at or past the end returns
// the state unchanged so the caller can render the finish screen.
func (s *SessionService) ShowQuizQuestion(
	ctx context.Context,
	sessionID,
	questionIdx int,
) (*model.QuizActiveSessionState, error) {
	state, err := s.quizProgress.Get(
		ctx,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	if questionIdx >= len(state.Items) {
		return state, nil
	}
	if questionIdx < 0 {
		return nil, fmt.Errorf(
			"move quiz cursor session_id=%d idx=%d: %w",
			sessionID,
			questionIdx,
			ErrQuizActiveSessionQuestionNotFound,
		)
	}
	state.CurrentIndex = questionIdx
	state.UpdatedAt = time.Now()
	if err := s.quizProgress.save(
		ctx,
		state,
	); err != nil {
		return nil, err
	}
	return state, nil
}
