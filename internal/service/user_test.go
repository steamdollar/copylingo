package service

import (
	"context"
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

type mockUserRepo struct {
	getOrCreateFn func(ctx context.Context, telegramID int64, username string) (*model.User, error)
	getAllUsersFn func(ctx context.Context) ([]model.User, error)
}

func (m *mockUserRepo) GetOrCreate(ctx context.Context, tid int64, user string) (*model.User, error) {
	return m.getOrCreateFn(ctx, tid, user)
}
func (m *mockUserRepo) GetAllUsers(ctx context.Context) ([]model.User, error) {
	return m.getAllUsersFn(ctx)
}

func TestGetUser(t *testing.T) {
	ctx := context.Background()
	tid := int64(12345)
	user := "testuser"

	mRepo := &mockUserRepo{
		getOrCreateFn: func(ctx context.Context, telegramID int64, username string) (*model.User, error) {
			if telegramID != tid || username != user {
				t.Errorf("unexpected args: %d, %s", telegramID, username)
			}
			return &model.User{ID: tid, Username: user}, nil
		},
	}

	svc := NewUserService(mRepo)
	u, err := svc.GetUser(ctx, tid, user)

	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	if u.ID != tid {
		t.Errorf("expected ID %d, got %d", tid, u.ID)
	}
}

func TestGetAllUsers(t *testing.T) {
	ctx := context.Background()

	mRepo := &mockUserRepo{
		getAllUsersFn: func(ctx context.Context) ([]model.User, error) {
			return []model.User{{ID: 1}, {ID: 2}}, nil
		},
	}

	svc := NewUserService(mRepo)
	users, err := svc.GetAllUsers(ctx)

	if err != nil {
		t.Fatalf("GetAllUsers failed: %v", err)
	}
	if len(users) != 2 {
		t.Errorf("expected 2 users, got %d", len(users))
	}
}
