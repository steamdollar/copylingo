package service

import (
	"context"
	"errors"
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

// fakeSessionRepo implements SessionRepo for SessionService tests. The
// embedded nil interface makes an unexpected repository call panic.
type fakeSessionRepo struct {
	SessionRepo
	oldest       *model.Session
	oldestUserID int64
	batchCounts  map[int64]int
	err          error
	startFn      func(id int) error
	created      []*model.Session
}

func (r *fakeSessionRepo) GetOldestUnfinished(
	_ context.Context,
	userID int64,
) (*model.Session, error) {
	r.oldestUserID = userID
	return r.oldest, r.err
}

func (r *fakeSessionRepo) CountUnfinishedBatch(
	context.Context,
	[]int64,
) (map[int64]int, error) {
	return r.batchCounts, r.err
}

func (r *fakeSessionRepo) Start(
	_ context.Context,
	id int,
) error {
	return r.startFn(id)
}

func (r *fakeSessionRepo) CreateSession(
	_ context.Context,
	s *model.Session,
) error {
	s.ID = 500 + len(r.created)
	r.created = append(
		r.created,
		s,
	)
	return nil
}

func TestSessionServiceOldestUnfinishedPassesThrough(t *testing.T) {
	want := &model.Session{ID: 42, UserID: 123, Status: model.SessionInProgress}
	repo := &fakeSessionRepo{oldest: want}
	svc := NewSessionService(SessionDeps{SessionRepo: repo})

	got, err := svc.OldestUnfinished(
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
	if repo.oldestUserID != want.UserID {
		t.Fatalf(
			"userID = %d, want %d",
			repo.oldestUserID,
			want.UserID,
		)
	}
}

func TestSessionServiceOldestUnfinishedReturnsRepositoryError(t *testing.T) {
	wantErr := errors.New("query failed")
	svc := NewSessionService(SessionDeps{SessionRepo: &fakeSessionRepo{err: wantErr}})

	_, err := svc.OldestUnfinished(
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
	svc := NewSessionService(SessionDeps{SessionRepo: &fakeSessionRepo{batchCounts: expected}})

	got, err := svc.CountUnfinishedBatch(
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
