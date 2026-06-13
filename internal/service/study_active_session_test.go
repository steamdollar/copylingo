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

func (f *fakeStudyActiveRepo) LoadStudyActiveSession(
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
				t.Fatalf("LoadStudyActiveSession sessionID = %d, want %d", gotSessionID, sessionID)
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
