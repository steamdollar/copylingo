package redisstore

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/lsj/copylingo/internal/model"
)

type fakeSessionRedis struct {
	values     map[string]string
	expiration map[string]time.Duration
	getErr     error
	setErr     error
	delErr     error
}

func newFakeSessionRedis() *fakeSessionRedis {
	return &fakeSessionRedis{values: make(map[string]string), expiration: make(map[string]time.Duration)}
}

func (f *fakeSessionRedis) Get(
	_ context.Context,
	key string,
) *redis.StringCmd {
	if f.getErr != nil {
		return redis.NewStringResult(
			"",
			f.getErr,
		)
	}
	value, ok := f.values[key]
	if !ok {
		return redis.NewStringResult(
			"",
			redis.Nil,
		)
	}
	return redis.NewStringResult(
		value,
		nil,
	)
}

func (f *fakeSessionRedis) Set(
	_ context.Context,
	key string,
	value any,
	expiration time.Duration,
) *redis.StatusCmd {
	if f.setErr != nil {
		return redis.NewStatusResult(
			"",
			f.setErr,
		)
	}
	switch value := value.(type) {
	case []byte:
		f.values[key] = string(value)
	case string:
		f.values[key] = value
	default:
		f.values[key] = fmt.Sprint(value)
	}
	f.expiration[key] = expiration
	return redis.NewStatusResult(
		"OK",
		nil,
	)
}

func (f *fakeSessionRedis) Del(
	_ context.Context,
	keys ...string,
) *redis.IntCmd {
	if f.delErr != nil {
		return redis.NewIntResult(
			0,
			f.delErr,
		)
	}
	var deleted int64
	for _, key := range keys {
		if _, ok := f.values[key]; ok {
			delete(
				f.values,
				key,
			)
			deleted++
		}
	}
	return redis.NewIntResult(
		deleted,
		nil,
	)
}

func TestQuizSessionsSaveLoadAndDelete(t *testing.T) {
	ctx := context.Background()
	rdb := newFakeSessionRedis()
	store := NewQuizSessions(rdb)
	state := &model.QuizActiveSessionState{Version: model.QuizActiveSessionStateVersion, Session: model.Session{ID: 41}}
	key := "session:41:working_set"

	if err := store.Save(
		ctx,
		41,
		state,
	); err != nil {
		t.Fatalf(
			"Save failed: %v",
			err,
		)
	}
	if got := rdb.expiration[key]; got != sessionWorkingSetTTL {
		t.Fatalf(
			"TTL = %s, want %s",
			got,
			sessionWorkingSetTTL,
		)
	}
	loaded, err := store.Load(
		ctx,
		41,
	)
	if err != nil {
		t.Fatalf(
			"Load failed: %v",
			err,
		)
	}
	if loaded.Session.ID != state.Session.ID || loaded.Version != state.Version {
		t.Fatalf(
			"loaded state = %+v, want session %d version %d",
			loaded,
			state.Session.ID,
			state.Version,
		)
	}
	if err := store.Delete(
		ctx,
		41,
	); err != nil {
		t.Fatalf(
			"Delete failed: %v",
			err,
		)
	}
	if _, err := store.Load(
		ctx,
		41,
	); !errors.Is(
		err,
		model.ErrSessionStoreNotFound,
	) {
		t.Fatalf(
			"Load after Delete = %v, want ErrSessionStoreNotFound",
			err,
		)
	}
}

func TestStudySessionsUsesExistingKeyAndTTL(t *testing.T) {
	ctx := context.Background()
	rdb := newFakeSessionRedis()
	store := NewStudySessions(rdb)
	key := "study_session:77:working_set"
	state := &model.StudyActiveSessionState{Session: model.Session{ID: 77}}

	if err := store.Save(
		ctx,
		77,
		state,
	); err != nil {
		t.Fatalf(
			"Save failed: %v",
			err,
		)
	}
	if rdb.values[key] == "" {
		t.Fatal("expected the existing study working-set key to be written")
	}
	if got := rdb.expiration[key]; got != sessionWorkingSetTTL {
		t.Fatalf(
			"TTL = %s, want %s",
			got,
			sessionWorkingSetTTL,
		)
	}
}

func TestSessionLoadCorruptJSONDeletesAndPreservesDeleteError(t *testing.T) {
	ctx := context.Background()
	rdb := newFakeSessionRedis()
	deleteErr := errors.New("redis delete failed")
	key := "session:12:working_set"
	rdb.values[key] = "{broken"
	rdb.delErr = deleteErr
	store := NewQuizSessions(rdb)

	_, err := store.Load(
		ctx,
		12,
	)
	if !errors.Is(
		err,
		model.ErrSessionStoreCorrupt,
	) || !errors.Is(
		err,
		deleteErr,
	) {
		t.Fatalf(
			"Load error = %v, want corrupt and delete errors",
			err,
		)
	}
	if _, ok := rdb.values[key]; !ok {
		t.Fatal("expected corrupt key to remain when deletion fails")
	}
}

func TestSessionLoadNullDeletesCorruptState(t *testing.T) {
	ctx := context.Background()
	rdb := newFakeSessionRedis()
	key := "session:13:working_set"
	rdb.values[key] = "null"
	store := NewQuizSessions(rdb)

	if _, err := store.Load(
		ctx,
		13,
	); !errors.Is(
		err,
		model.ErrSessionStoreCorrupt,
	) {
		t.Fatalf(
			"Load null JSON error = %v, want corrupt state",
			err,
		)
	}
	if _, ok := rdb.values[key]; ok {
		t.Fatal("corrupt null JSON should be deleted")
	}
}

func TestSessionLoadInvalidStateDeletesKey(t *testing.T) {
	ctx := context.Background()
	rdb := newFakeSessionRedis()
	quizStore := NewQuizSessions(rdb)
	studyStore := NewStudySessions(rdb)
	quizKey := "session:14:working_set"
	studyKey := "study_session:15:working_set"

	// Both session kinds reject snapshots that cannot belong to the requested key.
	if err := quizStore.Save(
		ctx,
		14,
		&model.QuizActiveSessionState{
			Version: model.QuizActiveSessionStateVersion + 1,
			Session: model.Session{ID: 14},
		},
	); err != nil {
		t.Fatal(err)
	}
	if err := studyStore.Save(
		ctx,
		15,
		&model.StudyActiveSessionState{
			Session: model.Session{ID: 16},
		},
	); err != nil {
		t.Fatal(err)
	}
	if _, err := quizStore.Load(
		ctx,
		14,
	); !errors.Is(
		err,
		model.ErrSessionStoreCorrupt,
	) {
		t.Fatalf(
			"Quiz Load error = %v, want corrupt state",
			err,
		)
	}
	if _, err := studyStore.Load(
		ctx,
		15,
	); !errors.Is(
		err,
		model.ErrSessionStoreCorrupt,
	) {
		t.Fatalf(
			"Study Load error = %v, want corrupt state",
			err,
		)
	}
	if _, ok := rdb.values[quizKey]; ok {
		t.Fatal("invalid Quiz key should be deleted")
	}
	if _, ok := rdb.values[studyKey]; ok {
		t.Fatal("invalid Study key should be deleted")
	}
}
