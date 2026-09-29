package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/lsj/copylingo/internal/model"
)

var (
	// ErrStudyFinalMarkFailed means the last material could not be marked
	// studied, so the Study session was not completed.
	ErrStudyFinalMarkFailed = errors.New("study final material mark failed")
	// ErrStudyCompleteFailed means every material was marked studied but the
	// session could not be completed.
	ErrStudyCompleteFailed = errors.New("study session complete failed")
)

// BuildStudy creates a Study session from the profile's plan; a positive
// limit scales the morning plan. nil means no materials were available.
func (s *SessionService) BuildStudy(
	ctx context.Context,
	user model.User,
	profile StudySessionProfile,
	limit int,
) (*model.Session, error) {
	return s.studyBuilder.BuildStudySession(
		ctx,
		user.ID,
		user.Language,
		user.ProficiencyLevel,
		profile,
		limit,
	)
}

// StartStudy marks the Study in_progress (when pending) and returns its
// working set, resuming Redis progress when present.
func (s *SessionService) StartStudy(
	ctx context.Context,
	sessionID int,
	userID int64,
) (*model.StudyActiveSessionState, error) {
	return s.studyProgress.Start(
		ctx,
		sessionID,
		userID,
	)
}

// MarkStudied records a material as studied in the Redis working set.
func (s *SessionService) MarkStudied(
	ctx context.Context,
	sessionID int,
	userID int64,
	materialOrder int,
) (*model.StudyActiveSessionState, error) {
	return s.studyProgress.MarkStudied(
		ctx,
		sessionID,
		userID,
		materialOrder,
	)
}

// FinishStudy marks the last shown material studied and completes the
// session. The returned error says which step failed so the caller can tell
// "progress not saved" from "completion not saved".
func (s *SessionService) FinishStudy(
	ctx context.Context,
	sessionID int,
	userID int64,
	lastMaterialOrder int,
) error {
	if _, err := s.studyProgress.MarkStudied(
		ctx,
		sessionID,
		userID,
		lastMaterialOrder,
	); err != nil {
		return fmt.Errorf(
			"%w session_id=%d material_order=%d: %w",
			ErrStudyFinalMarkFailed,
			sessionID,
			lastMaterialOrder,
			err,
		)
	}
	if err := s.CompleteStudy(
		ctx,
		sessionID,
		userID,
	); err != nil {
		return fmt.Errorf(
			"%w: %w",
			ErrStudyCompleteFailed,
			err,
		)
	}
	return nil
}

// CompleteStudy flushes a fully studied working set to DB and deletes it.
// Study completion does not update the streak (Quiz completion does).
func (s *SessionService) CompleteStudy(
	ctx context.Context,
	sessionID int,
	userID int64,
) error {
	return s.studyProgress.Complete(
		ctx,
		sessionID,
		userID,
	)
}

// StudyProgress returns the caller-owned Study working set, recovering it from DB when Redis has none.
func (s *SessionService) StudyProgress(
	ctx context.Context,
	sessionID int,
	userID int64,
) (*model.StudyActiveSessionState, error) {
	return s.studyProgress.LoadOwnedStudySessionState(
		ctx,
		sessionID,
		userID,
	)
}
