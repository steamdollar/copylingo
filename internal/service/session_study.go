package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/lsj/copylingo/internal/model"
)

var (
	ErrStudyFinalMarkFailed = errors.New("study final material mark failed")
	ErrStudyCompleteFailed  = errors.New("study session complete failed")
)

// A positive limit scales the morning plan; nil means no materials were available.
func (service *SessionService) BuildStudy(
	ctx context.Context,
	user model.User,
	profile StudySessionProfile,
	limit int,
) (*model.Session, error) {
	return service.studyBuilder.BuildStudySession(
		ctx,
		user.ID,
		user.Language,
		user.ProficiencyLevel,
		profile,
		limit,
	)
}

// StartStudy resumes Redis progress when present.
func (service *SessionService) StartStudy(
	ctx context.Context,
	sessionID int,
	userID int64,
) (*model.StudyActiveSessionState, error) {
	return service.studyProgress.Start(
		ctx,
		sessionID,
		userID,
	)
}

func (service *SessionService) MarkStudied(
	ctx context.Context,
	sessionID int,
	userID int64,
	materialOrder int,
) (*model.StudyActiveSessionState, error) {
	return service.studyProgress.MarkStudied(
		ctx,
		sessionID,
		userID,
		materialOrder,
	)
}

// FinishStudy's error says which step failed: "progress not saved" vs "completion not saved".
func (service *SessionService) FinishStudy(
	ctx context.Context,
	sessionID int,
	userID int64,
	lastMaterialOrder int,
) error {
	if _, err := service.studyProgress.MarkStudied(
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
	if err := service.CompleteStudy(
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

// Study completion does not update the streak (Quiz completion does).
func (service *SessionService) CompleteStudy(
	ctx context.Context,
	sessionID int,
	userID int64,
) error {
	return service.studyProgress.Complete(
		ctx,
		sessionID,
		userID,
	)
}

// StudyProgress recovers the working set from DB when Redis has none.
func (service *SessionService) StudyProgress(
	ctx context.Context,
	sessionID int,
	userID int64,
) (*model.StudyActiveSessionState, error) {
	return service.studyProgress.LoadOwnedStudySessionState(
		ctx,
		sessionID,
		userID,
	)
}
