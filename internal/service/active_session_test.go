package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
)

type fakeActiveSessionRedis struct {
	values map[string]string
	getErr error
	setErr error
	delErr error
}

func newFakeActiveSessionRedis() *fakeActiveSessionRedis {
	return &fakeActiveSessionRedis{values: make(map[string]string)}
}

func (f *fakeActiveSessionRedis) Get(ctx context.Context, key string) *redis.StringCmd {
	if f.getErr != nil {
		return redis.NewStringResult("", f.getErr)
	}
	val, ok := f.values[key]
	if !ok {
		return redis.NewStringResult("", redis.Nil)
	}
	return redis.NewStringResult(val, nil)
}

func (f *fakeActiveSessionRedis) Set(ctx context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd {
	if f.setErr != nil {
		return redis.NewStatusResult("", f.setErr)
	}
	switch v := value.(type) {
	case []byte:
		f.values[key] = string(v)
	case string:
		f.values[key] = v
	default:
		f.values[key] = fmt.Sprint(v)
	}
	return redis.NewStatusResult("OK", nil)
}

func (f *fakeActiveSessionRedis) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	if f.delErr != nil {
		return redis.NewIntResult(0, f.delErr)
	}
	var deleted int64
	for _, key := range keys {
		if _, ok := f.values[key]; ok {
			delete(f.values, key)
			deleted++
		}
	}
	return redis.NewIntResult(deleted, nil)
}

type fakeActiveSessionRepo struct {
	loadFn  func(ctx context.Context, sessionID int) (*model.ActiveSessionState, error)
	flushFn func(ctx context.Context, state *model.ActiveSessionState) error
}

func (f *fakeActiveSessionRepo) LoadActiveSession(ctx context.Context, sessionID int) (*model.ActiveSessionState, error) {
	return f.loadFn(ctx, sessionID)
}

func (f *fakeActiveSessionRepo) FlushActiveSession(ctx context.Context, state *model.ActiveSessionState) error {
	return f.flushFn(ctx, state)
}

func TestActiveSessionCreateFromDBAndGet(t *testing.T) {
	ctx := context.Background()
	sessionID := 10
	rdb := newFakeActiveSessionRedis()
	repo := &fakeActiveSessionRepo{
		loadFn: func(ctx context.Context, sid int) (*model.ActiveSessionState, error) {
			if sid != sessionID {
				t.Fatalf("unexpected session id %d", sid)
			}
			// One answered, one unanswered
			state := activeSessionTestState(sessionID, 123, true)
			state.Items = append(state.Items, model.ActiveSessionQuestion{
				SessionQuestion: model.SessionQuestion{ID: 101, QuestionID: 2, IsCorrect: nil},
				Question:        model.Question{ID: 2},
			})
			return state, nil
		},
	}
	svc := NewActiveSessionService(repo, rdb, NewSRSService(nil))

	if _, err := svc.CreateFromDB(ctx, sessionID); err != nil {
		t.Fatalf("CreateFromDB failed: %v", err)
	}
	got, err := svc.Get(ctx, sessionID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Session.ID != sessionID || len(got.Items) != 2 {
		t.Fatalf("unexpected state: %+v", got)
	}
	if got.CurrentIndex != 1 {
		t.Fatalf("expected CurrentIndex 1 (first unanswered), got %d", got.CurrentIndex)
	}
}

func TestActiveSessionGetAutoRecover(t *testing.T) {
	ctx := context.Background()
	sessionID := 10
	rdb := newFakeActiveSessionRedis() // Empty Redis
	repo := &fakeActiveSessionRepo{
		loadFn: func(ctx context.Context, sid int) (*model.ActiveSessionState, error) {
			return activeSessionTestState(sessionID, 123, false), nil
		},
	}
	svc := NewActiveSessionService(repo, rdb, nil)

	// Get should trigger auto-recovery because it's missing in Redis
	got, err := svc.Get(ctx, sessionID)
	if err != nil {
		t.Fatalf("Get failed to auto-recover: %v", err)
	}
	if got.Session.ID != sessionID {
		t.Fatalf("expected session id %d, got %d", sessionID, got.Session.ID)
	}

	// Verify it's now in Redis
	raw, err := rdb.Get(ctx, config.ActiveSessionWorkingSetRedisKey.Format(sessionID)).Result()
	if err != nil {
		t.Fatalf("expected state in Redis after auto-recovery: %v", err)
	}
	if raw == "" {
		t.Fatal("expected non-empty state in Redis")
	}
}

func TestActiveSessionGetMissing(t *testing.T) {
	ctx := context.Background()
	svc := NewActiveSessionService(nil, newFakeActiveSessionRedis(), nil)

	_, err := svc.Get(ctx, 10)
	if !errors.Is(err, ErrActiveSessionNotFound) {
		t.Fatalf("expected ErrActiveSessionNotFound, got %v", err)
	}
}

func TestActiveSessionGetCorruptDeletesKey(t *testing.T) {
	ctx := context.Background()
	sessionID := 10
	rdb := newFakeActiveSessionRedis()
	rdb.values[config.ActiveSessionWorkingSetRedisKey.Format(sessionID)] = "{broken"
	svc := NewActiveSessionService(nil, rdb, nil)

	_, err := svc.Get(ctx, sessionID)
	if !errors.Is(err, ErrActiveSessionCorrupt) {
		t.Fatalf("expected ErrActiveSessionCorrupt, got %v", err)
	}
	if _, ok := rdb.values[config.ActiveSessionWorkingSetRedisKey.Format(sessionID)]; ok {
		t.Fatal("expected corrupt key to be deleted")
	}
}

func TestActiveSessionRecordAnswerUpdatesProgressAndSRS(t *testing.T) {
	ctx := context.Background()
	sessionID := 10
	rdb := newFakeActiveSessionRedis()
	svc := NewActiveSessionService(nil, rdb, NewSRSService(nil))
	if err := svc.save(ctx, activeSessionTestState(sessionID, 123, false)); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	if err := svc.RecordAnswer(ctx, sessionID, 1, "apple", true); err != nil {
		t.Fatalf("RecordAnswer failed: %v", err)
	}

	got, err := svc.Get(ctx, sessionID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	item := got.Items[0]
	if got.AnsweredCount != 1 {
		t.Fatalf("expected answered count 1, got %d", got.AnsweredCount)
	}
	if item.SessionQuestion.UserAnswer == nil || *item.SessionQuestion.UserAnswer != "apple" {
		t.Fatalf("unexpected answer: %+v", item.SessionQuestion.UserAnswer)
	}
	if item.SessionQuestion.IsCorrect == nil || !*item.SessionQuestion.IsCorrect {
		t.Fatalf("expected correct answer, got %+v", item.SessionQuestion.IsCorrect)
	}
	if item.Question.TimesServed != 1 || item.Question.TimesCorrect != 1 {
		t.Fatalf("expected stats to increment, got served=%d correct=%d", item.Question.TimesServed, item.Question.TimesCorrect)
	}
	if item.Question.NextReviewAt == nil || item.Question.LastReviewedAt == nil {
		t.Fatal("expected SRS timestamps to be set")
	}
}

func TestActiveSessionRecordAnswerRejectsDuplicate(t *testing.T) {
	ctx := context.Background()
	sessionID := 10
	rdb := newFakeActiveSessionRedis()
	svc := NewActiveSessionService(nil, rdb, NewSRSService(nil))
	if err := svc.save(ctx, activeSessionTestState(sessionID, 123, true)); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	err := svc.RecordAnswer(ctx, sessionID, 1, "apple", true)
	if !errors.Is(err, ErrActiveSessionAlreadyAnswered) {
		t.Fatalf("expected ErrActiveSessionAlreadyAnswered, got %v", err)
	}
}

func TestActiveSessionRecordAnswerUsesCurrentDuplicateOccurrence(t *testing.T) {
	ctx := context.Background()
	sessionID := 10
	rdb := newFakeActiveSessionRedis()
	svc := NewActiveSessionService(nil, rdb, NewSRSService(nil))

	state := activeSessionTestState(sessionID, 123, true)
	state.Items = append(state.Items, model.ActiveSessionQuestion{
		SessionQuestion: model.SessionQuestion{ID: 101, SessionID: sessionID, QuestionID: 1},
		Question: model.Question{
			ID:            1,
			Type:          model.QuestionMultipleChoice,
			CorrectAnswer: "apple",
			EaseFactor:    2.5,
		},
	})
	state.CurrentIndex = 1
	if err := svc.save(ctx, state); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	if err := svc.RecordAnswer(ctx, sessionID, 1, "apple", true); err != nil {
		t.Fatalf("RecordAnswer failed: %v", err)
	}

	got, err := svc.Get(ctx, sessionID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Items[1].SessionQuestion.IsCorrect == nil || !*got.Items[1].SessionQuestion.IsCorrect {
		t.Fatalf("expected current duplicate occurrence to be answered: %+v", got.Items[1].SessionQuestion)
	}
}

func TestActiveSessionRecordAnswerRejectsStaleDuplicateCallback(t *testing.T) {
	ctx := context.Background()
	sessionID := 10
	rdb := newFakeActiveSessionRedis()
	svc := NewActiveSessionService(nil, rdb, NewSRSService(nil))

	state := activeSessionTestState(sessionID, 123, true)
	state.Items = append(state.Items, model.ActiveSessionQuestion{
		SessionQuestion: model.SessionQuestion{ID: 101, SessionID: sessionID, QuestionID: 1},
		Question: model.Question{
			ID:            1,
			Type:          model.QuestionMultipleChoice,
			CorrectAnswer: "apple",
			EaseFactor:    2.5,
		},
	})
	if err := svc.save(ctx, state); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	err := svc.RecordAnswer(ctx, sessionID, 1, "apple", true)
	if !errors.Is(err, ErrActiveSessionAlreadyAnswered) {
		t.Fatalf("expected ErrActiveSessionAlreadyAnswered, got %v", err)
	}

	got, getErr := svc.Get(ctx, sessionID)
	if getErr != nil {
		t.Fatalf("Get failed: %v", getErr)
	}
	if got.Items[1].SessionQuestion.IsCorrect != nil {
		t.Fatalf("expected later duplicate occurrence to remain unanswered: %+v", got.Items[1].SessionQuestion)
	}
}

func TestActiveSessionFlushSuccess(t *testing.T) {
	ctx := context.Background()
	sessionID := 10
	userID := int64(123)
	rdb := newFakeActiveSessionRedis()
	flushed := false
	repo := &fakeActiveSessionRepo{
		flushFn: func(ctx context.Context, state *model.ActiveSessionState) error {
			flushed = true
			if state.Session.CorrectCount != 1 {
				t.Fatalf("expected correct count 1, got %d", state.Session.CorrectCount)
			}
			return nil
		},
	}
	svc := NewActiveSessionService(repo, rdb, NewSRSService(nil))
	if err := svc.save(ctx, activeSessionTestState(sessionID, userID, true)); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	result, err := svc.Flush(ctx, sessionID, userID)
	if err != nil {
		t.Fatalf("Flush failed: %v", err)
	}
	if !flushed {
		t.Fatal("expected repository flush")
	}
	if result.TotalQuestions != 1 || result.CorrectCount != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestActiveSessionFlushRejectsIncomplete(t *testing.T) {
	ctx := context.Background()
	sessionID := 10
	userID := int64(123)
	rdb := newFakeActiveSessionRedis()
	svc := NewActiveSessionService(&fakeActiveSessionRepo{}, rdb, NewSRSService(nil))
	if err := svc.save(ctx, activeSessionTestState(sessionID, userID, false)); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	_, err := svc.Flush(ctx, sessionID, userID)
	if !errors.Is(err, ErrActiveSessionIncomplete) {
		t.Fatalf("expected ErrActiveSessionIncomplete, got %v", err)
	}
}

func activeSessionTestState(sessionID int, userID int64, answered bool) *model.ActiveSessionState {
	state := activeStateForQuestion(sessionID, model.Question{
		ID:            1,
		Type:          model.QuestionMultipleChoice,
		CorrectAnswer: "apple",
		EaseFactor:    2.5,
	}, answered)
	state.Session.UserID = userID
	state.Items[0].Question.EaseFactor = 2.5
	return state
}
