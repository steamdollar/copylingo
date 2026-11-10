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
const maxReviewQuizQuestions = 10

var (
	// ErrNoDueReviews means an on-demand review Quiz has nothing to review.
	ErrNoDueReviews = errors.New("no due review questions")
	// ErrQuizStatePrepareFailed means the Quiz was marked started in DB but
	// its Redis working set could not be reloaded to reflect that.
	ErrQuizStatePrepareFailed = errors.New("quiz working set prepare failed")
)

// BuildMorningQuiz creates the morning Quiz session; nil means no questions were available.
func (service *SessionService) BuildMorningQuiz(
	ctx context.Context,
	user model.User,
) (*model.Session, error) {
	return service.selection.BuildMorningSession(
		ctx,
		user.ID,
		user.Language,
		user.ProficiencyLevel,
	)
}

// BuildEveningQuiz creates the evening Quiz session; nil means no questions were available.
func (service *SessionService) BuildEveningQuiz(
	ctx context.Context,
	user model.User,
) (*model.Session, error) {
	return service.selection.BuildEveningSession(
		ctx,
		user.ID,
		user.Language,
		user.ProficiencyLevel,
	)
}

// BuildReviewQuiz creates an on-demand review Quiz of at most
// maxReviewQuizQuestions due items. A due-count lookup failure is treated as
// zero due items (existing policy), so it surfaces as ErrNoDueReviews.
func (service *SessionService) BuildReviewQuiz(
	ctx context.Context,
	user model.User,
) (*model.Session, error) {
	dueCount, err := service.DueReviewCount(
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
	return service.selection.BuildReviewSession(
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
func (service *SessionService) StartQuiz(
	ctx context.Context,
	sessionID int,
	userID int64,
) (*model.QuizActiveSessionState, error) {
	state, err := service.quizProgress.Get(
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

	if err := service.sessionRepository.Start(
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
	state, err = service.quizProgress.CreateFromDB(
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

// ShowQuizQuestion moves the working-set cursor to questionIndex and returns the
// state to render, with one Redis read. An index at or past the end returns
// the state unchanged so the caller can render the finish screen.
func (service *SessionService) ShowQuizQuestion(
	ctx context.Context,
	sessionID,
	questionIndex int,
) (*model.QuizActiveSessionState, error) {
	state, err := service.quizProgress.Get(
		ctx,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	if questionIndex >= len(state.Items) {
		return state, nil
	}
	if questionIndex < 0 {
		return nil, fmt.Errorf(
			"move quiz cursor session_id=%d idx=%d: %w",
			sessionID,
			questionIndex,
			ErrQuizActiveSessionQuestionNotFound,
		)
	}
	state.CurrentIndex = questionIndex
	state.UpdatedAt = time.Now()
	if err := service.quizProgress.save(
		ctx,
		state,
	); err != nil {
		return nil, err
	}
	return state, nil
}
