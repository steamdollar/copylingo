package service

import (
	"context"
	"errors"
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

type unfinishedSessionRepoStub struct {
	session *model.Session
	err     error
	userID  int64
}

func (r *unfinishedSessionRepoStub) GetOldestUnfinished(ctx context.Context, userID int64) (*model.Session, error) {
	r.userID = userID
	return r.session, r.err
}

func TestSessionQueryGetOldestUnfinishedPassesThrough(t *testing.T) {
	want := &model.Session{ID: 42, UserID: 123, Status: model.SessionInProgress}
	repo := &unfinishedSessionRepoStub{session: want}
	svc := NewSessionQueryService(repo)

	got, err := svc.GetOldestUnfinished(context.Background(), want.UserID)
	if err != nil {
		t.Fatalf("GetOldestUnfinished failed: %v", err)
	}
	if got != want {
		t.Fatalf("session = %+v, want same pointer %+v", got, want)
	}
	if repo.userID != want.UserID {
		t.Fatalf("userID = %d, want %d", repo.userID, want.UserID)
	}
}

func TestSessionQueryGetOldestUnfinishedReturnsRepositoryError(t *testing.T) {
	wantErr := errors.New("query failed")
	svc := NewSessionQueryService(&unfinishedSessionRepoStub{err: wantErr})

	_, err := svc.GetOldestUnfinished(context.Background(), 123)
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
}
