package service

import (
	"context"
	"errors"
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

// fakeSessionRepository implements SessionRepository for SessionService tests. The
// embedded nil interface makes an unexpected repository call panic.
type fakeSessionRepository struct {
	SessionRepository
	oldest       *model.Session
	oldestUserID int64
	batchCounts  map[int64]int
	err          error
	startFunc    func(id int) error
	created      []*model.Session
}

func (fake *fakeSessionRepository) GetOldestUnfinished(
	_ context.Context,
	userID int64,
) (*model.Session, error) {
	fake.oldestUserID = userID
	return fake.oldest, fake.err
}

func (fake *fakeSessionRepository) CountUnfinishedBatch(
	context.Context,
	[]int64,
) (map[int64]int, error) {
	return fake.batchCounts, fake.err
}

func (fake *fakeSessionRepository) Start(
	_ context.Context,
	id int,
) error {
	return fake.startFunc(id)
}

func (fake *fakeSessionRepository) CreateSession(
	_ context.Context,
	session *model.Session,
) error {
	session.ID = 500 + len(fake.created)
	fake.created = append(
		fake.created,
		session,
	)
	return nil
}

func TestSessionServiceOldestUnfinishedPassesThrough(t *testing.T) {
	want := &model.Session{ID: 42, UserID: 123, Status: model.SessionInProgress}
	repository := &fakeSessionRepository{oldest: want}
	sessionService := NewSessionService(SessionDependencies{SessionRepository: repository})

	got, err := sessionService.OldestUnfinished(
		context.Background(),
		want.UserID,
	)
	if err != nil {
		t.Fatalf(
			"OldestUnfinished failed: %v",
			err,
		)
	}
	if got != want {
		t.Fatalf(
			"session = %+v, want same pointer %+v",
			got,
			want,
		)
	}
	if repository.oldestUserID != want.UserID {
		t.Fatalf(
			"userID = %d, want %d",
			repository.oldestUserID,
			want.UserID,
		)
	}
}

func TestSessionServiceOldestUnfinishedReturnsRepositoryError(t *testing.T) {
	wantErr := errors.New("query failed")
	sessionService := NewSessionService(SessionDependencies{SessionRepository: &fakeSessionRepository{err: wantErr}})

	_, err := sessionService.OldestUnfinished(
		context.Background(),
		123,
	)
	if !errors.Is(
		err,
		wantErr,
	) {
		t.Fatalf(
			"error = %v, want %v",
			err,
			wantErr,
		)
	}
}

func TestSessionServiceCountUnfinishedBatchPassesThrough(t *testing.T) {
	expected := map[int64]int{1: 2, 2: 0}
	sessionService := NewSessionService(
		SessionDependencies{SessionRepository: &fakeSessionRepository{batchCounts: expected}},
	)

	got, err := sessionService.CountUnfinishedBatch(
		context.Background(),
		[]int64{1, 2},
	)
	if err != nil {
		t.Fatalf(
			"CountUnfinishedBatch failed: %v",
			err,
		)
	}
	if len(got) != 2 || got[1] != 2 || got[2] != 0 {
		t.Fatalf(
			"counts = %+v, want %+v",
			got,
			expected,
		)
	}
}
