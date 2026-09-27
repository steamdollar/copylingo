package service

import (
	"context"
	"errors"
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

type preferenceRepoStub struct {
	calls, offset, limit int
	userID               int64
	items                []model.MaterialPreference
	err                  error
}

func (r *preferenceRepoStub) Get(_ context.Context, userID int64, materialID int) (*model.MaterialPreference, error) {
	r.calls++
	return &model.MaterialPreference{
		UserID:     userID,
		MaterialID: materialID,
		ReviewMode: model.MaterialReviewNormal,
	}, r.err
}

func (r *preferenceRepoStub) Set(_ context.Context, userID int64, materialID int, mode model.MaterialReviewMode) error {
	r.calls++
	r.userID = userID
	return r.err
}

func (r *preferenceRepoStub) List(
	_ context.Context,
	userID int64,
	offset, limit int,
) ([]model.MaterialPreference, error) {
	r.calls++
	r.userID, r.offset, r.limit = userID, offset, limit
	return r.items, r.err
}

func TestMaterialPreferenceRejectsInvalidRequests(t *testing.T) {
	r := &preferenceRepoStub{}
	s := NewMaterialPreferenceService(r)
	ctx := context.Background()
	for _, input := range []struct {
		userID int64
		id     int
		mode   model.MaterialReviewMode
	}{
		{0, 1, model.MaterialReviewNormal},
		{1, 0, model.MaterialReviewNormal},
		{1, 1, "unknown"},
	} {
		if err := s.Set(ctx, input.userID, input.id, input.mode); err == nil {
			t.Fatalf("invalid set accepted: %+v", input)
		}
	}
	if _, err := s.Get(ctx, 1, -1); err == nil {
		t.Fatal("invalid material accepted")
	}
	for _, page := range []int{-1, int(^uint(0) >> 1)} {
		if _, _, err := s.List(ctx, 1, page); err == nil {
			t.Fatalf("invalid page %d accepted", page)
		}
	}
	if r.calls != 0 {
		t.Fatal("invalid requests reached repository")
	}
}

func TestMaterialPreferencePaginationPreservesUserScope(t *testing.T) {
	r := &preferenceRepoStub{items: make([]model.MaterialPreference, MaterialPreferencePageSize+1)}
	s := NewMaterialPreferenceService(r)
	items, more, err := s.List(context.Background(), 42, 2)
	if err != nil || !more || len(items) != MaterialPreferencePageSize {
		t.Fatalf("page len=%d more=%v err=%v", len(items), more, err)
	}
	if r.userID != 42 || r.offset != 16 || r.limit != 9 {
		t.Fatalf("query scope: %+v", r)
	}
	r.items = r.items[:3]
	items, more, err = s.List(context.Background(), 42, 3)
	if err != nil || more || len(items) != 3 {
		t.Fatalf("last page len=%d more=%v err=%v", len(items), more, err)
	}
	r.err = errors.New("repository unavailable")
	if _, _, err = s.List(context.Background(), 42, 0); !errors.Is(err, r.err) {
		t.Fatalf("repository error lost: %v", err)
	}
}
