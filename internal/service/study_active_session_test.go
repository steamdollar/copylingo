package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
)

type fakeStudyActiveRepo struct {
	loadFn  func(ctx context.Context, sessionID int) (*model.StudyActiveSessionState, error)
	flushFn func(ctx context.Context, state *model.StudyActiveSessionState) error
}

func (f *fakeStudyActiveRepo) LoadStudySessionWithStateBySessionID(
	ctx context.Context,
	sessionID int,
) (*model.StudyActiveSessionState, error) {
	return f.loadFn(ctx, sessionID)
}

func (f *fakeStudyActiveRepo) FlushStudyActiveSession(ctx context.Context, state *model.StudyActiveSessionState) error {
	return f.flushFn(ctx, state)
}

type fakeStudySessionStarter struct {
	started []int
}

func (f *fakeStudySessionStarter) Start(ctx context.Context, id int) error {
	f.started = append(f.started, id)
	return nil
}

func TestStudyActiveSessionStartLoadsAndStoresWorkingSet(t *testing.T) {
	ctx := context.Background()
	sessionID := 77
	userID := int64(123)
	rdb := newFakeActiveSessionRedis()
	starter := &fakeStudySessionStarter{}
	repo := &fakeStudyActiveRepo{
		loadFn: func(ctx context.Context, gotSessionID int) (*model.StudyActiveSessionState, error) {
			if gotSessionID != sessionID {
				t.Fatalf("LoadStudySessionWithStateBySessionID sessionID = %d, want %d", gotSessionID, sessionID)
			}
			return studyActiveState(sessionID, userID, model.SessionPending), nil
		},
	}
	svc := NewStudyActiveSessionService(repo, starter, rdb)

	state, err := svc.Start(ctx, sessionID, userID)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if state.Session.Status != model.SessionInProgress {
		t.Fatalf("status = %s, want %s", state.Session.Status, model.SessionInProgress)
	}
	if len(starter.started) != 1 || starter.started[0] != sessionID {
		t.Fatalf("started = %+v, want [%d]", starter.started, sessionID)
	}
	if _, err := rdb.Get(ctx, config.StudySessionWorkingSetRedisKey.Format(sessionID)).Result(); err != nil {
		t.Fatalf("working set missing after Start: %v", err)
	}
}

func TestStudyActiveSessionStartResumesRedisWorkingSet(t *testing.T) {
	ctx := context.Background()
	sessionID := 77
	userID := int64(123)
	rdb := newFakeActiveSessionRedis()
	starter := &fakeStudySessionStarter{}
	loadCalls := 0
	repo := &fakeStudyActiveRepo{
		loadFn: func(ctx context.Context, gotSessionID int) (*model.StudyActiveSessionState, error) {
			loadCalls++
			return studyActiveState(sessionID, userID, model.SessionPending), nil
		},
	}
	svc := NewStudyActiveSessionService(repo, starter, rdb)

	if _, err := svc.Start(ctx, sessionID, userID); err != nil {
		t.Fatalf("initial Start failed: %v", err)
	}
	if _, err := svc.MarkStudied(ctx, sessionID, userID, 0); err != nil {
		t.Fatalf("MarkStudied failed: %v", err)
	}

	resumed, err := svc.Start(ctx, sessionID, userID)
	if err != nil {
		t.Fatalf("resumed Start failed: %v", err)
	}
	if loadCalls != 1 {
		t.Fatalf("DB load calls = %d, want 1 after Redis resume", loadCalls)
	}
	if len(starter.started) != 1 {
		t.Fatalf("starter calls = %+v, want one initial DB start", starter.started)
	}
	if resumed.Items[0].SessionMaterial.StudiedAt == nil {
		t.Fatal("expected previously studied material to survive repeated Start")
	}
	if got := resumed.NextUnstudiedIndex(); got != 1 {
		t.Fatalf("next unstudied index = %d, want 1", got)
	}
	if resumed.CurrentIndex != 1 {
		t.Fatalf("current index = %d, want 1 on resume", resumed.CurrentIndex)
	}
}

func TestStudyActiveSessionMarkStudiedUpdatesRedisOnly(t *testing.T) {
	ctx := context.Background()
	sessionID := 77
	userID := int64(123)
	rdb := newFakeActiveSessionRedis()
	svc := NewStudyActiveSessionService(nil, nil, rdb)
	if err := svc.save(ctx, studyActiveState(sessionID, userID, model.SessionInProgress)); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	state, err := svc.MarkStudied(ctx, sessionID, userID, 0)
	if err != nil {
		t.Fatalf("MarkStudied failed: %v", err)
	}
	if state.Items[0].SessionMaterial.StudiedAt == nil {
		t.Fatal("expected first material to be studied")
	}
	got, err := svc.Get(ctx, sessionID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Items[0].SessionMaterial.StudiedAt == nil {
		t.Fatal("expected studied state to be persisted in Redis")
	}
}

func TestStudyActiveSessionCompleteFlushesAndDeletesWorkingSet(t *testing.T) {
	ctx := context.Background()
	sessionID := 77
	userID := int64(123)
	rdb := newFakeActiveSessionRedis()
	flushed := false
	repo := &fakeStudyActiveRepo{
		flushFn: func(ctx context.Context, state *model.StudyActiveSessionState) error {
			flushed = true
			if len(state.NewlyStudiedMaterialIDs()) != 2 {
				t.Fatalf("NewlyStudiedMaterialIDs = %+v, want 2 ids", state.NewlyStudiedMaterialIDs())
			}
			return nil
		},
	}
	svc := NewStudyActiveSessionService(repo, nil, rdb)
	state := studyActiveState(sessionID, userID, model.SessionInProgress)
	now := time.Now()
	state.MarkStudied(0, now)
	state.MarkStudied(1, now)
	if err := svc.save(ctx, state); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	if err := svc.Complete(ctx, sessionID, userID); err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	if !flushed {
		t.Fatal("expected repository flush")
	}
	if _, err := rdb.Get(ctx, config.StudySessionWorkingSetRedisKey.Format(sessionID)).
		Result(); !errors.Is(
		err,
		redis.Nil,
	) {
		t.Fatalf("working set should be deleted, got err=%v", err)
	}
}

func TestStudyActiveSessionCompleteRejectsIncomplete(t *testing.T) {
	ctx := context.Background()
	sessionID := 77
	userID := int64(123)
	rdb := newFakeActiveSessionRedis()
	svc := NewStudyActiveSessionService(&fakeStudyActiveRepo{}, nil, rdb)
	if err := svc.save(ctx, studyActiveState(sessionID, userID, model.SessionInProgress)); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	err := svc.Complete(ctx, sessionID, userID)
	if !errors.Is(err, ErrStudyActiveSessionIncomplete) {
		t.Fatalf("expected ErrStudyActiveSessionIncomplete, got %v", err)
	}
}

func TestStudyActiveSessionStartReturnsCompletedEarly(t *testing.T) {
	ctx := context.Background()
	sessionID := 77
	userID := int64(123)
	rdb := newFakeActiveSessionRedis()
	starter := &fakeStudySessionStarter{}
	repo := &fakeStudyActiveRepo{
		loadFn: func(ctx context.Context, sid int) (*model.StudyActiveSessionState, error) {
			return studyActiveState(sessionID, userID, model.SessionCompleted), nil
		},
	}
	svc := NewStudyActiveSessionService(repo, starter, rdb)

	state, err := svc.Start(ctx, sessionID, userID)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if state.Session.Status != model.SessionCompleted {
		t.Fatalf("status = %s, want completed", state.Session.Status)
	}
	// Completed sessions must not be (re)started.
	if len(starter.started) != 0 {
		t.Fatalf("started = %+v, want empty for completed session", starter.started)
	}
}

func TestStudyActiveSessionStartPendingWithoutStarterFails(t *testing.T) {
	ctx := context.Background()
	sessionID := 77
	userID := int64(123)
	rdb := newFakeActiveSessionRedis()
	repo := &fakeStudyActiveRepo{
		loadFn: func(ctx context.Context, sid int) (*model.StudyActiveSessionState, error) {
			return studyActiveState(sessionID, userID, model.SessionPending), nil
		},
	}
	// sessionRepo nil while session is pending -> dependency missing.
	svc := NewStudyActiveSessionService(repo, nil, rdb)

	_, err := svc.Start(ctx, sessionID, userID)
	if !errors.Is(err, ErrStudyActiveSessionDependencyMissing) {
		t.Fatalf("Start pending without starter = %v, want ErrStudyActiveSessionDependencyMissing", err)
	}
}

func TestStudyActiveSessionStartRejectsUserMismatch(t *testing.T) {
	ctx := context.Background()
	sessionID := 77
	rdb := newFakeActiveSessionRedis()
	repo := &fakeStudyActiveRepo{
		loadFn: func(ctx context.Context, sid int) (*model.StudyActiveSessionState, error) {
			return studyActiveState(sessionID, 123, model.SessionInProgress), nil
		},
	}
	svc := NewStudyActiveSessionService(repo, &fakeStudySessionStarter{}, rdb)

	_, err := svc.Start(ctx, sessionID, 999)
	if !errors.Is(err, ErrStudyActiveSessionUserMismatch) {
		t.Fatalf("Start user mismatch = %v, want ErrStudyActiveSessionUserMismatch", err)
	}
}

func TestStudyActiveSessionStartDependencyMissingRepo(t *testing.T) {
	ctx := context.Background()
	// repo nil -> loadFromDB returns dependency missing.
	svc := NewStudyActiveSessionService(nil, nil, newFakeActiveSessionRedis())
	_, err := svc.Start(ctx, 77, 123)
	if !errors.Is(err, ErrStudyActiveSessionDependencyMissing) {
		t.Fatalf("Start nil repo = %v, want ErrStudyActiveSessionDependencyMissing", err)
	}
}

func TestStudyActiveSessionCreateFromDB(t *testing.T) {
	ctx := context.Background()
	sessionID := 77
	userID := int64(123)
	rdb := newFakeActiveSessionRedis()
	repo := &fakeStudyActiveRepo{
		loadFn: func(ctx context.Context, sid int) (*model.StudyActiveSessionState, error) {
			return studyActiveState(sessionID, userID, model.SessionInProgress), nil
		},
	}
	svc := NewStudyActiveSessionService(repo, nil, rdb)

	state, err := svc.CreateFromDB(ctx, sessionID)
	if err != nil {
		t.Fatalf("CreateFromDB failed: %v", err)
	}
	if state.Session.ID != sessionID {
		t.Fatalf("session id = %d, want %d", state.Session.ID, sessionID)
	}
	// Must be persisted to Redis.
	if _, err := rdb.Get(ctx, config.StudySessionWorkingSetRedisKey.Format(sessionID)).Result(); err != nil {
		t.Fatalf("working set missing after CreateFromDB: %v", err)
	}
}

func TestStudyActiveSessionCreateFromDBPropagatesLoadError(t *testing.T) {
	ctx := context.Background()
	loadErr := errors.New("db load failed")
	repo := &fakeStudyActiveRepo{
		loadFn: func(ctx context.Context, sid int) (*model.StudyActiveSessionState, error) {
			return nil, loadErr
		},
	}
	svc := NewStudyActiveSessionService(repo, nil, newFakeActiveSessionRedis())

	if _, err := svc.CreateFromDB(ctx, 77); !errors.Is(err, loadErr) {
		t.Fatalf("CreateFromDB load error = %v, want %v in chain", err, loadErr)
	}
}

func TestStudyActiveSessionGetRecoversFromDB(t *testing.T) {
	ctx := context.Background()
	sessionID := 77
	userID := int64(123)
	rdb := newFakeActiveSessionRedis() // empty -> not found -> recover
	repo := &fakeStudyActiveRepo{
		loadFn: func(ctx context.Context, sid int) (*model.StudyActiveSessionState, error) {
			return studyActiveState(sessionID, userID, model.SessionInProgress), nil
		},
	}
	svc := NewStudyActiveSessionService(repo, nil, rdb)

	state, err := svc.Get(ctx, sessionID)
	if err != nil {
		t.Fatalf("Get auto-recover failed: %v", err)
	}
	if state.Session.ID != sessionID {
		t.Fatalf("session id = %d, want %d", state.Session.ID, sessionID)
	}
}

func TestStudyActiveSessionGetOwnedLoadsFromDBOnMiss(t *testing.T) {
	ctx := context.Background()
	sessionID := 77
	userID := int64(123)
	rdb := newFakeActiveSessionRedis() // empty -> GetOwned loads from DB
	repo := &fakeStudyActiveRepo{
		loadFn: func(ctx context.Context, sid int) (*model.StudyActiveSessionState, error) {
			return studyActiveState(sessionID, userID, model.SessionInProgress), nil
		},
	}
	svc := NewStudyActiveSessionService(repo, nil, rdb)

	state, err := svc.GetOwned(ctx, sessionID, userID)
	if err != nil {
		t.Fatalf("GetOwned failed: %v", err)
	}
	if state.Session.ID != sessionID {
		t.Fatalf("session id = %d, want %d", state.Session.ID, sessionID)
	}
	// GetOwned persists the loaded state back to Redis.
	if _, err := rdb.Get(ctx, config.StudySessionWorkingSetRedisKey.Format(sessionID)).Result(); err != nil {
		t.Fatalf("working set missing after GetOwned: %v", err)
	}
}

func TestStudyActiveSessionGetOwnedRejectsUserMismatch(t *testing.T) {
	ctx := context.Background()
	sessionID := 77
	rdb := newFakeActiveSessionRedis()
	svc := NewStudyActiveSessionService(&fakeStudyActiveRepo{}, nil, rdb)
	if err := svc.save(ctx, studyActiveState(sessionID, 123, model.SessionInProgress)); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	_, err := svc.GetOwned(ctx, sessionID, 999)
	if !errors.Is(err, ErrStudyActiveSessionUserMismatch) {
		t.Fatalf("GetOwned user mismatch = %v, want ErrStudyActiveSessionUserMismatch", err)
	}
}

func TestStudyActiveSessionGetOwnedRejectsModeMismatch(t *testing.T) {
	ctx := context.Background()
	sessionID := 77
	userID := int64(123)
	rdb := newFakeActiveSessionRedis()
	svc := NewStudyActiveSessionService(&fakeStudyActiveRepo{}, nil, rdb)

	state := studyActiveState(sessionID, userID, model.SessionInProgress)
	state.Session.Mode = model.SessionModeQuiz // wrong mode for a study session
	if err := svc.save(ctx, state); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	_, err := svc.GetOwned(ctx, sessionID, userID)
	if !errors.Is(err, ErrStudyActiveSessionModeMismatch) {
		t.Fatalf("GetOwned mode mismatch = %v, want ErrStudyActiveSessionModeMismatch", err)
	}
}

func TestStudyActiveSessionMarkStudiedRejectsUnknownOrder(t *testing.T) {
	ctx := context.Background()
	sessionID := 77
	userID := int64(123)
	rdb := newFakeActiveSessionRedis()
	svc := NewStudyActiveSessionService(nil, nil, rdb)
	if err := svc.save(ctx, studyActiveState(sessionID, userID, model.SessionInProgress)); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	_, err := svc.MarkStudied(ctx, sessionID, userID, 99)
	if !errors.Is(err, ErrStudyActiveSessionMaterialNotFound) {
		t.Fatalf("MarkStudied unknown order = %v, want ErrStudyActiveSessionMaterialNotFound", err)
	}
}

func TestStudyActiveSessionDelete(t *testing.T) {
	ctx := context.Background()
	sessionID := 77
	key := config.StudySessionWorkingSetRedisKey.Format(sessionID)
	rdb := newFakeActiveSessionRedis()
	svc := NewStudyActiveSessionService(nil, nil, rdb)
	if err := svc.save(ctx, studyActiveState(sessionID, 123, model.SessionInProgress)); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	if err := svc.Delete(ctx, sessionID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if _, ok := rdb.values[key]; ok {
		t.Fatal("expected key removed after Delete")
	}
}

func studyActiveState(sessionID int, userID int64, status model.SessionStatus) *model.StudyActiveSessionState {
	state := &model.StudyActiveSessionState{
		Version: model.StudyActiveSessionStateVersion,
		Session: model.Session{
			ID:     sessionID,
			UserID: userID,
			Type:   model.SessionStudy,
			Mode:   model.SessionModeStudy,
			Status: status,
		},
		Items: []model.StudySessionMaterial{
			studyActiveItem(sessionID, 10, 0),
			studyActiveItem(sessionID, 11, 1),
		},
		UpdatedAt: time.Now(),
	}
	state.CaptureInitiallyStudied()
	state.RecountStudied()
	return state
}

func studyActiveItem(sessionID, materialID, order int) model.StudySessionMaterial {
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
