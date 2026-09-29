package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lsj/copylingo/internal/model"
)

var (
	ErrStudyActiveSessionMaterialNotFound  = errors.New("study active session material not found")
	ErrStudyActiveSessionIncomplete        = errors.New("study active session is incomplete")
	ErrStudyActiveSessionUserMismatch      = errors.New("study active session user mismatch")
	ErrStudyActiveSessionModeMismatch      = errors.New("study active session mode mismatch")
	ErrStudyActiveSessionDependencyMissing = errors.New("study active session dependency missing")
)

type StudySessionStore interface {
	Load(
		ctx context.Context,
		sessionID int,
	) (*model.StudyActiveSessionState, error)
	Save(
		ctx context.Context,
		sessionID int,
		state *model.StudyActiveSessionState,
	) error
	Delete(
		ctx context.Context,
		sessionID int,
	) error
}

type studyActiveSessionRepository interface {
	LoadStudySessionWithStateBySessionID(
		ctx context.Context,
		sessionID int,
	) (*model.StudyActiveSessionState, error)
	FlushStudyActiveSession(
		ctx context.Context,
		state *model.StudyActiveSessionState,
	) error
}

type studyActiveSessionStarter interface {
	Start(
		ctx context.Context,
		id int,
	) error
}

// studyActiveSessionService owns the Redis working set for in-progress study sessions.
type studyActiveSessionService struct {
	repo        studyActiveSessionRepository
	sessionRepo studyActiveSessionStarter
	store       StudySessionStore
}

func newStudyActiveSessionService(
	repo studyActiveSessionRepository,
	sessionRepo studyActiveSessionStarter,
	store StudySessionStore,
) *studyActiveSessionService {
	return &studyActiveSessionService{
		repo:        repo,
		sessionRepo: sessionRepo,
		store:       store,
	}
}

func (s *studyActiveSessionService) Start(
	ctx context.Context,
	sessionID int,
	userID int64,
) (*model.StudyActiveSessionState, error) {
	// Resume from the Redis working set when present. Loading from DB here
	// would discard materials marked studied since the session was started.
	if s.store == nil {
		return nil, ErrStudyActiveSessionDependencyMissing
	}
	state, err := s.store.Load(
		ctx,
		sessionID,
	)
	if errors.Is(
		err,
		model.ErrSessionStoreNotFound,
	) {
		state, err = s.loadFromDB(
			ctx,
			sessionID,
		)
	}
	if err != nil {
		return nil, err
	}
	state.RecountStudied()
	if err := validateStudyOwnerAndMode(
		state,
		sessionID,
		userID,
	); err != nil {
		return nil, err
	}
	if state.Session.Status == model.SessionCompleted {
		return state, nil
	}
	state.CurrentIndex = state.NextUnstudiedIndex()
	if state.Session.Status == model.SessionPending {
		if s.sessionRepo == nil {
			return nil, ErrStudyActiveSessionDependencyMissing
		}
		if err := s.sessionRepo.Start(
			ctx,
			sessionID,
		); err != nil {
			return nil, fmt.Errorf(
				"start study active session session_id=%d: %w",
				sessionID,
				err,
			)
		}
		state.Session.Status = model.SessionInProgress
		state.UpdatedAt = time.Now()
	}
	if err := s.save(
		ctx,
		state,
	); err != nil {
		return nil, err
	}
	return state, nil
}

func (s *studyActiveSessionService) loadFromDB(
	ctx context.Context,
	sessionID int,
) (*model.StudyActiveSessionState, error) {
	if s.repo == nil {
		return nil, ErrStudyActiveSessionDependencyMissing
	}
	state, err := s.repo.LoadStudySessionWithStateBySessionID(
		ctx,
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create study active session from db session_id=%d: %w",
			sessionID,
			err,
		)
	}
	return state, nil
}

func (s *studyActiveSessionService) LoadOwnedStudySessionState(
	ctx context.Context,
	sessionID int,
	userID int64,
) (*model.StudyActiveSessionState, error) {
	if s.store == nil {
		return nil, ErrStudyActiveSessionDependencyMissing
	}
	state, err := s.store.Load(
		ctx,
		sessionID,
	)
	if err != nil {
		if !errors.Is(
			err,
			model.ErrSessionStoreNotFound,
		) {
			return nil, err
		}
		// no cache > db hit
		state, err = s.loadFromDB(
			ctx,
			sessionID,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"%w session_id=%d: %v",
				model.ErrSessionStoreNotFound,
				sessionID,
				err,
			)
		}
	}
	// validate
	if err := validateStudyOwnerAndMode(
		state,
		sessionID,
		userID,
	); err != nil {
		return nil, err
	}

	// redis에 save
	if err := s.save(
		ctx,
		state,
	); err != nil {
		return nil, err
	}
	return state, nil
}

func (s *studyActiveSessionService) MarkStudied(
	ctx context.Context,
	sessionID int,
	userID int64,
	materialOrder int,
) (*model.StudyActiveSessionState, error) {
	// get session state by ID
	state, err := s.LoadOwnedStudySessionState(
		ctx,
		sessionID,
		userID,
	)
	if err != nil {
		return nil, err
	}

	//
	if _, _, ok := state.ItemByOrder(materialOrder); !ok {
		return nil, fmt.Errorf(
			"%w session_id=%d material_order=%d",
			ErrStudyActiveSessionMaterialNotFound,
			sessionID,
			materialOrder,
		)
	}
	state.MarkStudied(
		materialOrder,
		time.Now(),
	)
	if err := s.save(
		ctx,
		state,
	); err != nil {
		return nil, err
	}
	return state, nil
}

func (s *studyActiveSessionService) Complete(
	ctx context.Context,
	sessionID int,
	userID int64,
) error {
	state, err := s.LoadOwnedStudySessionState(
		ctx,
		sessionID,
		userID,
	)
	if err != nil {
		return err
	}
	if state.NextUnstudiedIndex() != len(state.Items) {
		return fmt.Errorf(
			"%w session_id=%d",
			ErrStudyActiveSessionIncomplete,
			sessionID,
		)
	}
	if s.repo == nil {
		return ErrStudyActiveSessionDependencyMissing
	}
	if err := s.repo.FlushStudyActiveSession(
		ctx,
		state,
	); err != nil {
		return fmt.Errorf(
			"flush study active session state session_id=%d: %w",
			sessionID,
			err,
		)
	}
	if err := s.Delete(
		ctx,
		sessionID,
	); err != nil {
		return err
	}
	return nil
}

func (s *studyActiveSessionService) Delete(
	ctx context.Context,
	sessionID int,
) error {
	if s.store == nil {
		return ErrStudyActiveSessionDependencyMissing
	}
	if err := s.store.Delete(
		ctx,
		sessionID,
	); err != nil {
		return fmt.Errorf(
			"delete study active session working set session_id=%d: %w",
			sessionID,
			err,
		)
	}
	return nil
}

func (s *studyActiveSessionService) save(
	ctx context.Context,
	state *model.StudyActiveSessionState,
) error {
	state.RecountStudied()
	if s.store == nil {
		return ErrStudyActiveSessionDependencyMissing
	}
	if err := s.store.Save(
		ctx,
		state.Session.ID,
		state,
	); err != nil {
		return fmt.Errorf(
			"set study active session working set session_id=%d: %w",
			state.Session.ID,
			err,
		)
	}
	return nil
}

func validateStudyOwnerAndMode(
	state *model.StudyActiveSessionState,
	sessionID int,
	userID int64,
) error {
	if state.Session.UserID != userID {
		return fmt.Errorf(
			"%w session_id=%d user_id=%d",
			ErrStudyActiveSessionUserMismatch,
			sessionID,
			userID,
		)
	}
	if state.Session.Mode != model.SessionModeStudy {
		return fmt.Errorf(
			"%w session_id=%d mode=%s",
			ErrStudyActiveSessionModeMismatch,
			sessionID,
			state.Session.Mode,
		)
	}
	return nil
}
