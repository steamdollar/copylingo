package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lsj/copylingo/internal/model"
)

type fakeStudySessionStore struct {
	values map[int]*model.StudyActiveSessionState
}

func newFakeStudySessionStore() *fakeStudySessionStore {
	return &fakeStudySessionStore{values: make(map[int]*model.StudyActiveSessionState)}
}

func (fake *fakeStudySessionStore) Load(
	_ context.Context,
	sessionID int,
) (*model.StudyActiveSessionState, error) {
	state, ok := fake.values[sessionID]
	if !ok {
		return nil, model.ErrSessionStoreNotFound
	}
	return snapshotSessionTestState(state)
}

func (fake *fakeStudySessionStore) Save(
	_ context.Context,
	sessionID int,
	state *model.StudyActiveSessionState,
) error {
	snapshot, err := snapshotSessionTestState(state)
	if err != nil {
		return err
	}
	fake.values[sessionID] = snapshot
	return nil
}

func (fake *fakeStudySessionStore) Delete(
	_ context.Context,
	sessionID int,
) error {
	delete(
		fake.values,
		sessionID,
	)
	return nil
}

type fakeStudyActiveRepository struct {
	loadFunc func(
		ctx context.Context,
		sessionID int,
	) (*model.StudyActiveSessionState, error)
	flushFunc func(
		ctx context.Context,
		state *model.StudyActiveSessionState,
	) error
}

func (fake *fakeStudyActiveRepository) LoadStudySessionWithStateBySessionID(
	ctx context.Context,
	sessionID int,
) (*model.StudyActiveSessionState, error) {
	return fake.loadFunc(
		ctx,
		sessionID,
	)
}

func (fake *fakeStudyActiveRepository) FlushStudyActiveSession(
	ctx context.Context,
	state *model.StudyActiveSessionState,
) error {
	return fake.flushFunc(
		ctx,
		state,
	)
}

type fakeStudySessionStarter struct {
	started []int
}

func (fake *fakeStudySessionStarter) Start(
	ctx context.Context,
	id int,
) error {
	fake.started = append(
		fake.started,
		id,
	)
	return nil
}

func TestStudyActiveSessionStartLoadsAndStoresWorkingSet(t *testing.T) {
	ctx := context.Background()
	sessionID := 77
	userID := int64(123)
	store := newFakeStudySessionStore()
	starter := &fakeStudySessionStarter{}
	repository := &fakeStudyActiveRepository{
		loadFunc: func(
			ctx context.Context,
			gotSessionID int,
		) (*model.StudyActiveSessionState, error) {
			if gotSessionID != sessionID {
				t.Fatalf(
					"LoadStudySessionWithStateBySessionID sessionID = %d, want %d",
					gotSessionID,
					sessionID,
				)
			}
			return studyActiveState(
				sessionID,
				userID,
				model.SessionPending,
			), nil
		},
	}
	service := newStudyActiveSessionService(
		repository,
		starter,
		store,
	)

	state, err := service.Start(
		ctx,
		sessionID,
		userID,
	)
	if err != nil {
		t.Fatalf(
			"Start failed: %v",
			err,
		)
	}
	if state.Session.Status != model.SessionInProgress {
		t.Fatalf(
			"status = %s, want %s",
			state.Session.Status,
			model.SessionInProgress,
		)
	}
	if len(starter.started) != 1 || starter.started[0] != sessionID {
		t.Fatalf(
			"started = %+v, want [%d]",
			starter.started,
			sessionID,
		)
	}
	if _, err := store.Load(
		ctx,
		sessionID,
	); err != nil {
		t.Fatalf(
			"working set missing after Start: %v",
			err,
		)
	}
}

func TestStudyActiveSessionStartResumesRedisWorkingSet(t *testing.T) {
	ctx := context.Background()
	sessionID := 77
	userID := int64(123)
	store := newFakeStudySessionStore()
	starter := &fakeStudySessionStarter{}
	loadCalls := 0
	repository := &fakeStudyActiveRepository{
		loadFunc: func(
			ctx context.Context,
			gotSessionID int,
		) (*model.StudyActiveSessionState, error) {
			loadCalls++
			return studyActiveState(
				sessionID,
				userID,
				model.SessionPending,
			), nil
		},
	}
	service := newStudyActiveSessionService(
		repository,
		starter,
		store,
	)

	if _, err := service.Start(
		ctx,
		sessionID,
		userID,
	); err != nil {
		t.Fatalf(
			"initial Start failed: %v",
			err,
		)
	}
	if _, err := service.MarkStudied(
		ctx,
		sessionID,
		userID,
		0,
	); err != nil {
		t.Fatalf(
			"MarkStudied failed: %v",
			err,
		)
	}

	resumed, err := service.Start(
		ctx,
		sessionID,
		userID,
	)
	if err != nil {
		t.Fatalf(
			"resumed Start failed: %v",
			err,
		)
	}
	if loadCalls != 1 {
		t.Fatalf(
			"DB load calls = %d, want 1 after Redis resume",
			loadCalls,
		)
	}
	if len(starter.started) != 1 {
		t.Fatalf(
			"starter calls = %+v, want one initial DB start",
			starter.started,
		)
	}
	if resumed.Items[0].SessionMaterial.StudiedAt == nil {
		t.Fatal("expected previously studied material to survive repeated Start")
	}
	if got := resumed.NextUnstudiedIndex(); got != 1 {
		t.Fatalf(
			"next unstudied index = %d, want 1",
			got,
		)
	}
	if resumed.CurrentIndex != 1 {
		t.Fatalf(
			"current index = %d, want 1 on resume",
			resumed.CurrentIndex,
		)
	}
}

func TestStudyActiveSessionMarkStudiedUpdatesRedisOnly(t *testing.T) {
	ctx := context.Background()
	sessionID := 77
	userID := int64(123)
	store := newFakeStudySessionStore()
	service := newStudyActiveSessionService(
		nil,
		nil,
		store,
	)
	if err := service.save(
		ctx,
		studyActiveState(
			sessionID,
			userID,
			model.SessionInProgress,
		),
	); err != nil {
		t.Fatalf(
			"save failed: %v",
			err,
		)
	}

	state, err := service.MarkStudied(
		ctx,
		sessionID,
		userID,
		0,
	)
	if err != nil {
		t.Fatalf(
			"MarkStudied failed: %v",
			err,
		)
	}
	if state.Items[0].SessionMaterial.StudiedAt == nil {
		t.Fatal("expected first material to be studied")
	}
	got, err := store.Load(
		ctx,
		sessionID,
	)
	if err != nil {
		t.Fatalf(
			"Get failed: %v",
			err,
		)
	}
	if got.Items[0].SessionMaterial.StudiedAt == nil {
		t.Fatal("expected studied state to be persisted in Redis")
	}
}

func TestStudyActiveSessionCompleteFlushesAndDeletesWorkingSet(t *testing.T) {
	ctx := context.Background()
	sessionID := 77
	userID := int64(123)
	store := newFakeStudySessionStore()
	flushed := false
	repository := &fakeStudyActiveRepository{
		flushFunc: func(
			ctx context.Context,
			state *model.StudyActiveSessionState,
		) error {
			flushed = true
			if len(state.NewlyStudiedMaterialIDs()) != 2 {
				t.Fatalf(
					"NewlyStudiedMaterialIDs = %+v, want 2 ids",
					state.NewlyStudiedMaterialIDs(),
				)
			}
			return nil
		},
	}
	service := newStudyActiveSessionService(
		repository,
		nil,
		store,
	)
	state := studyActiveState(
		sessionID,
		userID,
		model.SessionInProgress,
	)
	now := time.Now()
	state.MarkStudied(
		0,
		now,
	)
	state.MarkStudied(
		1,
		now,
	)
	if err := service.save(
		ctx,
		state,
	); err != nil {
		t.Fatalf(
			"save failed: %v",
			err,
		)
	}

	if err := service.Complete(
		ctx,
		sessionID,
		userID,
	); err != nil {
		t.Fatalf(
			"Complete failed: %v",
			err,
		)
	}
	if !flushed {
		t.Fatal("expected repository flush")
	}
	if _, ok := store.values[sessionID]; ok {
		t.Fatal("working set should be deleted")
	}
}

func TestStudyActiveSessionCompleteRejectsIncomplete(t *testing.T) {
	ctx := context.Background()
	sessionID := 77
	userID := int64(123)
	store := newFakeStudySessionStore()
	service := newStudyActiveSessionService(
		&fakeStudyActiveRepository{},
		nil,
		store,
	)
	if err := service.save(
		ctx,
		studyActiveState(
			sessionID,
			userID,
			model.SessionInProgress,
		),
	); err != nil {
		t.Fatalf(
			"save failed: %v",
			err,
		)
	}

	err := service.Complete(
		ctx,
		sessionID,
		userID,
	)
	if !errors.Is(
		err,
		ErrStudyActiveSessionIncomplete,
	) {
		t.Fatalf(
			"expected ErrStudyActiveSessionIncomplete, got %v",
			err,
		)
	}
}

func TestStudyActiveSessionStartReturnsCompletedEarly(t *testing.T) {
	ctx := context.Background()
	sessionID := 77
	userID := int64(123)
	store := newFakeStudySessionStore()
	starter := &fakeStudySessionStarter{}
	repository := &fakeStudyActiveRepository{
		loadFunc: func(
			ctx context.Context,
			requestedSessionID int,
		) (*model.StudyActiveSessionState, error) {
			return studyActiveState(
				sessionID,
				userID,
				model.SessionCompleted,
			), nil
		},
	}
	service := newStudyActiveSessionService(
		repository,
		starter,
		store,
	)

	state, err := service.Start(
		ctx,
		sessionID,
		userID,
	)
	if err != nil {
		t.Fatalf(
			"Start failed: %v",
			err,
		)
	}
	if state.Session.Status != model.SessionCompleted {
		t.Fatalf(
			"status = %s, want completed",
			state.Session.Status,
		)
	}
	// Completed sessions must not be (re)started.
	if len(starter.started) != 0 {
		t.Fatalf(
			"started = %+v, want empty for completed session",
			starter.started,
		)
	}
}

func TestStudyActiveSessionStartRejectsUserMismatch(t *testing.T) {
	ctx := context.Background()
	sessionID := 77
	store := newFakeStudySessionStore()
	repository := &fakeStudyActiveRepository{
		loadFunc: func(
			ctx context.Context,
			requestedSessionID int,
		) (*model.StudyActiveSessionState, error) {
			return studyActiveState(
				sessionID,
				123,
				model.SessionInProgress,
			), nil
		},
	}
	service := newStudyActiveSessionService(
		repository,
		&fakeStudySessionStarter{},
		store,
	)

	_, err := service.Start(
		ctx,
		sessionID,
		999,
	)
	if !errors.Is(
		err,
		ErrStudyActiveSessionUserMismatch,
	) {
		t.Fatalf(
			"Start user mismatch = %v, want ErrStudyActiveSessionUserMismatch",
			err,
		)
	}
}

func TestStudyActiveSessionStartPropagatesLoadError(t *testing.T) {
	ctx := context.Background()
	loadErr := errors.New("db load failed")
	repository := &fakeStudyActiveRepository{
		loadFunc: func(
			ctx context.Context,
			sessionID int,
		) (*model.StudyActiveSessionState, error) {
			return nil, loadErr
		},
	}
	service := newStudyActiveSessionService(
		repository,
		nil,
		newFakeStudySessionStore(),
	)

	if _, err := service.Start(
		ctx,
		77,
		123,
	); !errors.Is(
		err,
		loadErr,
	) {
		t.Fatalf(
			"Start load error = %v, want %v in chain",
			err,
			loadErr,
		)
	}
}

func TestStudyActiveSessionLoadOwnedStudySessionStateLoadsFromDBOnMiss(t *testing.T) {
	ctx := context.Background()
	sessionID := 77
	userID := int64(123)
	store := newFakeStudySessionStore() // empty -> LoadOwnedStudySessionState loads from DB
	repository := &fakeStudyActiveRepository{
		loadFunc: func(
			ctx context.Context,
			requestedSessionID int,
		) (*model.StudyActiveSessionState, error) {
			return studyActiveState(
				sessionID,
				userID,
				model.SessionInProgress,
			), nil
		},
	}
	service := newStudyActiveSessionService(
		repository,
		nil,
		store,
	)

	state, err := service.LoadOwnedStudySessionState(
		ctx,
		sessionID,
		userID,
	)
	if err != nil {
		t.Fatalf(
			"LoadOwnedStudySessionState failed: %v",
			err,
		)
	}
	if state.Session.ID != sessionID {
		t.Fatalf(
			"session id = %d, want %d",
			state.Session.ID,
			sessionID,
		)
	}
	// LoadOwnedStudySessionState persists the loaded state back to Redis.
	if _, err := store.Load(
		ctx,
		sessionID,
	); err != nil {
		t.Fatalf(
			"working set missing after LoadOwnedStudySessionState: %v",
			err,
		)
	}
}

func TestStudyActiveSessionLoadOwnedStudySessionStateRejectsUserMismatch(t *testing.T) {
	ctx := context.Background()
	sessionID := 77
	store := newFakeStudySessionStore()
	service := newStudyActiveSessionService(
		&fakeStudyActiveRepository{},
		nil,
		store,
	)
	if err := service.save(
		ctx,
		studyActiveState(
			sessionID,
			123,
			model.SessionInProgress,
		),
	); err != nil {
		t.Fatalf(
			"save failed: %v",
			err,
		)
	}

	_, err := service.LoadOwnedStudySessionState(
		ctx,
		sessionID,
		999,
	)
	if !errors.Is(
		err,
		ErrStudyActiveSessionUserMismatch,
	) {
		t.Fatalf(
			"LoadOwnedStudySessionState user mismatch = %v, want ErrStudyActiveSessionUserMismatch",
			err,
		)
	}
}

func TestStudyActiveSessionLoadOwnedStudySessionStateRejectsModeMismatch(t *testing.T) {
	ctx := context.Background()
	sessionID := 77
	userID := int64(123)
	store := newFakeStudySessionStore()
	service := newStudyActiveSessionService(
		&fakeStudyActiveRepository{},
		nil,
		store,
	)

	state := studyActiveState(
		sessionID,
		userID,
		model.SessionInProgress,
	)
	state.Session.Mode = model.SessionModeQuiz // wrong mode for a study session
	if err := service.save(
		ctx,
		state,
	); err != nil {
		t.Fatalf(
			"save failed: %v",
			err,
		)
	}

	_, err := service.LoadOwnedStudySessionState(
		ctx,
		sessionID,
		userID,
	)
	if !errors.Is(
		err,
		ErrStudyActiveSessionModeMismatch,
	) {
		t.Fatalf(
			"LoadOwnedStudySessionState mode mismatch = %v, want ErrStudyActiveSessionModeMismatch",
			err,
		)
	}
}

func TestStudyActiveSessionMarkStudiedRejectsUnknownOrder(t *testing.T) {
	ctx := context.Background()
	sessionID := 77
	userID := int64(123)
	store := newFakeStudySessionStore()
	service := newStudyActiveSessionService(
		nil,
		nil,
		store,
	)
	if err := service.save(
		ctx,
		studyActiveState(
			sessionID,
			userID,
			model.SessionInProgress,
		),
	); err != nil {
		t.Fatalf(
			"save failed: %v",
			err,
		)
	}

	_, err := service.MarkStudied(
		ctx,
		sessionID,
		userID,
		99,
	)
	if !errors.Is(
		err,
		ErrStudyActiveSessionMaterialNotFound,
	) {
		t.Fatalf(
			"MarkStudied unknown order = %v, want ErrStudyActiveSessionMaterialNotFound",
			err,
		)
	}
}

func TestStudyActiveSessionDelete(t *testing.T) {
	ctx := context.Background()
	sessionID := 77
	store := newFakeStudySessionStore()
	service := newStudyActiveSessionService(
		nil,
		nil,
		store,
	)
	if err := service.save(
		ctx,
		studyActiveState(
			sessionID,
			123,
			model.SessionInProgress,
		),
	); err != nil {
		t.Fatalf(
			"save failed: %v",
			err,
		)
	}

	if err := service.Delete(
		ctx,
		sessionID,
	); err != nil {
		t.Fatalf(
			"Delete failed: %v",
			err,
		)
	}
	if _, ok := store.values[sessionID]; ok {
		t.Fatal("expected key removed after Delete")
	}
}

func studyActiveState(
	sessionID int,
	userID int64,
	status model.SessionStatus,
) *model.StudyActiveSessionState {
	state := &model.StudyActiveSessionState{
		Session: model.Session{
			ID:     sessionID,
			UserID: userID,
			Type:   model.SessionStudy,
			Mode:   model.SessionModeStudy,
			Status: status,
		},
		Items: []model.StudySessionMaterial{
			studyActiveItem(
				sessionID,
				10,
				0,
			),
			studyActiveItem(
				sessionID,
				11,
				1,
			),
		},
		UpdatedAt: time.Now(),
	}
	state.CaptureInitiallyStudied()
	state.RecountStudied()
	return state
}

func studyActiveItem(
	sessionID,
	materialID,
	order int,
) model.StudySessionMaterial {
	return model.StudySessionMaterial{
		SessionMaterial: model.SessionMaterial{
			ID:            materialID + 1000,
			SessionID:     sessionID,
			MaterialID:    materialID,
			MaterialOrder: order,
		},
		Material: model.Material{ID: materialID},
	}
}
