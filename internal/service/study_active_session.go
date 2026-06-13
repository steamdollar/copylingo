package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
)

const studyActiveSessionWorkingSetTTL = 24 * time.Hour

var (
	ErrStudyActiveSessionNotFound          = errors.New("study active session state not found")
	ErrStudyActiveSessionCorrupt           = errors.New("study active session state is corrupt")
	ErrStudyActiveSessionMaterialNotFound  = errors.New("study active session material not found")
	ErrStudyActiveSessionIncomplete        = errors.New("study active session is incomplete")
	ErrStudyActiveSessionUserMismatch      = errors.New("study active session user mismatch")
	ErrStudyActiveSessionModeMismatch      = errors.New("study active session mode mismatch")
	ErrStudyActiveSessionDependencyMissing = errors.New("study active session dependency missing")
)

type studyActiveSessionRepository interface {
	LoadStudyActiveSession(ctx context.Context, sessionID int) (*model.StudyActiveSessionState, error)
	FlushStudyActiveSession(ctx context.Context, state *model.StudyActiveSessionState) error
}

type studyActiveSessionStarter interface {
	Start(ctx context.Context, id int) error
}

// StudyActiveSessionService owns the Redis working set for in-progress study sessions.
type StudyActiveSessionService struct {
	repo        studyActiveSessionRepository
	sessionRepo studyActiveSessionStarter
	workingSet  *workingSetStore[model.StudyActiveSessionState]
}

func NewStudyActiveSessionService(
	repo studyActiveSessionRepository,
	sessionRepo studyActiveSessionStarter,
	rdb workingSetRedis,
) *StudyActiveSessionService {
	return &StudyActiveSessionService{
		repo:        repo,
		sessionRepo: sessionRepo,
		workingSet: newWorkingSetStore(
			rdb,
			studyActiveSessionWorkingSetKey,
			studyActiveSessionWorkingSetTTL,
			validateStudyActiveSessionState,
			workingSetErrors{
				DependencyMissing: ErrStudyActiveSessionDependencyMissing,
				NotFound:          ErrStudyActiveSessionNotFound,
				Corrupt:           ErrStudyActiveSessionCorrupt,
			},
		),
	}
}

func (s *StudyActiveSessionService) Start(
	ctx context.Context,
	sessionID int,
	userID int64,
) (*model.StudyActiveSessionState, error) {
	state, err := s.loadFromDB(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if err := validateStudyOwnerAndMode(state, sessionID, userID); err != nil {
		return nil, err
	}
	if state.Session.Status == model.SessionCompleted {
		return state, nil
	}
	if state.Session.Status == model.SessionPending {
		if s.sessionRepo == nil {
			return nil, ErrStudyActiveSessionDependencyMissing
		}
		if err := s.sessionRepo.Start(ctx, sessionID); err != nil {
			return nil, fmt.Errorf("start study active session session_id=%d: %w", sessionID, err)
		}
		state.Session.Status = model.SessionInProgress
		state.UpdatedAt = time.Now()
	}
	if err := s.save(ctx, state); err != nil {
		return nil, err
	}
	return state, nil
}

func (s *StudyActiveSessionService) CreateFromDB(
	ctx context.Context,
	sessionID int,
) (*model.StudyActiveSessionState, error) {
	state, err := s.loadFromDB(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if err := s.save(ctx, state); err != nil {
		return nil, fmt.Errorf("store study active session working set session_id=%d: %w", sessionID, err)
	}
	return state, nil
}

func (s *StudyActiveSessionService) loadFromDB(
	ctx context.Context,
	sessionID int,
) (*model.StudyActiveSessionState, error) {
	if s.repo == nil {
		return nil, ErrStudyActiveSessionDependencyMissing
	}
	state, err := s.repo.LoadStudyActiveSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("create study active session from db session_id=%d: %w", sessionID, err)
	}
	state.Version = model.StudyActiveSessionStateVersion
	state.UpdatedAt = time.Now()
	state.RecountStudied()
	state.CurrentIndex = state.NextUnstudiedIndex()
	if state.InitiallyStudiedMaterialIDs == nil {
		state.CaptureInitiallyStudied()
	}
	return state, nil
}

func (s *StudyActiveSessionService) Get(ctx context.Context, sessionID int) (*model.StudyActiveSessionState, error) {
	state, err := s.workingSet.get(ctx, sessionID)
	if err != nil {
		if errors.Is(err, ErrStudyActiveSessionNotFound) {
			state, err := s.CreateFromDB(ctx, sessionID)
			if err != nil {
				return nil, fmt.Errorf("%w session_id=%d: %v", ErrStudyActiveSessionNotFound, sessionID, err)
			}
			return state, nil
		}
		return nil, err
	}
	state.RecountStudied()
	return state, nil
}

func (s *StudyActiveSessionService) GetOwned(
	ctx context.Context,
	sessionID int,
	userID int64,
) (*model.StudyActiveSessionState, error) {
	state, err := s.workingSet.get(ctx, sessionID)
	if err != nil {
		if !errors.Is(err, ErrStudyActiveSessionNotFound) {
			return nil, err
		}
		state, err = s.loadFromDB(ctx, sessionID)
		if err != nil {
			return nil, fmt.Errorf("%w session_id=%d: %v", ErrStudyActiveSessionNotFound, sessionID, err)
		}
	}
	if err := validateStudyOwnerAndMode(state, sessionID, userID); err != nil {
		return nil, err
	}
	state.RecountStudied()
	if err := s.save(ctx, state); err != nil {
		return nil, err
	}
	return state, nil
}

func (s *StudyActiveSessionService) MarkStudied(
	ctx context.Context,
	sessionID int,
	userID int64,
	materialOrder int,
) (*model.StudyActiveSessionState, error) {
	state, err := s.GetOwned(ctx, sessionID, userID)
	if err != nil {
		return nil, err
	}
	if _, _, ok := state.ItemByOrder(materialOrder); !ok {
		return nil, fmt.Errorf("%w session_id=%d material_order=%d",
			ErrStudyActiveSessionMaterialNotFound, sessionID, materialOrder)
	}
	state.MarkStudied(materialOrder, time.Now())
	if err := s.save(ctx, state); err != nil {
		return nil, err
	}
	return state, nil
}

func (s *StudyActiveSessionService) Complete(ctx context.Context, sessionID int, userID int64) error {
	state, err := s.GetOwned(ctx, sessionID, userID)
	if err != nil {
		return err
	}
	if state.NextUnstudiedIndex() != len(state.Items) {
		return fmt.Errorf("%w session_id=%d", ErrStudyActiveSessionIncomplete, sessionID)
	}
	if s.repo == nil {
		return ErrStudyActiveSessionDependencyMissing
	}
	if err := s.repo.FlushStudyActiveSession(ctx, state); err != nil {
		return fmt.Errorf("flush study active session state session_id=%d: %w", sessionID, err)
	}
	if err := s.Delete(ctx, sessionID); err != nil {
		return err
	}
	return nil
}

func (s *StudyActiveSessionService) Delete(ctx context.Context, sessionID int) error {
	if err := s.workingSet.delete(ctx, sessionID); err != nil {
		return fmt.Errorf("delete study active session working set session_id=%d: %w", sessionID, err)
	}
	return nil
}

func (s *StudyActiveSessionService) save(ctx context.Context, state *model.StudyActiveSessionState) error {
	state.RecountStudied()
	if err := s.workingSet.save(ctx, state.Session.ID, state); err != nil {
		return fmt.Errorf("set study active session working set session_id=%d: %w", state.Session.ID, err)
	}
	return nil
}

func studyActiveSessionWorkingSetKey(sessionID int) string {
	return config.StudySessionWorkingSetRedisKey.Format(sessionID)
}

func validateStudyActiveSessionState(state *model.StudyActiveSessionState, sessionID int) error {
	if state.Version != model.StudyActiveSessionStateVersion || state.Session.ID != sessionID {
		return ErrStudyActiveSessionCorrupt
	}
	return nil
}

func validateStudyOwnerAndMode(state *model.StudyActiveSessionState, sessionID int, userID int64) error {
	if state.Session.UserID != userID {
		return fmt.Errorf("%w session_id=%d user_id=%d", ErrStudyActiveSessionUserMismatch, sessionID, userID)
	}
	if state.Session.Mode != model.SessionModeStudy {
		return fmt.Errorf("%w session_id=%d mode=%s", ErrStudyActiveSessionModeMismatch, sessionID, state.Session.Mode)
	}
	return nil
}
