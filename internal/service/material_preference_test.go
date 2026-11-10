package service

import (
	"context"
	"errors"
	"testing"

	"github.com/lsj/copylingo/internal/model"
)

type preferenceRepositoryStub struct {
	calls, offset, limit int
	userID               int64
	items                []model.MaterialPreference
	err                  error
}

func (stub *preferenceRepositoryStub) Get(
	_ context.Context,
	userID int64,
	materialID int,
) (*model.MaterialPreference, error) {
	stub.calls++
	return &model.MaterialPreference{
		UserID:     userID,
		MaterialID: materialID,
		ReviewMode: model.MaterialReviewNormal,
	}, stub.err
}

func (stub *preferenceRepositoryStub) Set(
	_ context.Context,
	userID int64,
	materialID int,
	mode model.MaterialReviewMode,
) error {
	stub.calls++
	stub.userID = userID
	return stub.err
}

func (stub *preferenceRepositoryStub) List(
	_ context.Context,
	userID int64,
	offset,
	limit int,
) ([]model.MaterialPreference, error) {
	stub.calls++
	stub.userID, stub.offset, stub.limit = userID, offset, limit
	return stub.items, stub.err
}

func TestMaterialPreferenceRejectsInvalidRequests(t *testing.T) {
	repository := &preferenceRepositoryStub{}
	service := NewMaterialPreferenceService(repository)
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
		if err := service.Set(
			ctx,
			input.userID,
			input.id,
			input.mode,
		); err == nil {
			t.Fatalf(
				"invalid set accepted: %+v",
				input,
			)
		}
	}
	if _, err := service.Get(
		ctx,
		1,
		-1,
	); err == nil {
		t.Fatal("invalid material accepted")
	}
	for _, page := range []int{-1, int(^uint(0) >> 1)} {
		if _, _, err := service.List(
			ctx,
			1,
			page,
		); err == nil {
			t.Fatalf(
				"invalid page %d accepted",
				page,
			)
		}
	}
	if repository.calls != 0 {
		t.Fatal("invalid requests reached repository")
	}
}

func TestMaterialPreferencePaginationPreservesUserScope(t *testing.T) {
	repository := &preferenceRepositoryStub{items: make(
		[]model.MaterialPreference,
		MaterialPreferencePageSize+1,
	)}
	service := NewMaterialPreferenceService(repository)
	items, more, err := service.List(
		context.Background(),
		42,
		2,
	)
	if err != nil || !more || len(items) != MaterialPreferencePageSize {
		t.Fatalf(
			"page len=%d more=%v err=%v",
			len(items),
			more,
			err,
		)
	}
	if repository.userID != 42 || repository.offset != 16 || repository.limit != 9 {
		t.Fatalf(
			"query scope: %+v",
			repository,
		)
	}
	repository.items = repository.items[:3]
	items, more, err = service.List(
		context.Background(),
		42,
		3,
	)
	if err != nil || more || len(items) != 3 {
		t.Fatalf(
			"last page len=%d more=%v err=%v",
			len(items),
			more,
			err,
		)
	}
	repository.err = errors.New("repository unavailable")
	if _, _, err = service.List(
		context.Background(),
		42,
		0,
	); !errors.Is(
		err,
		repository.err,
	) {
		t.Fatalf(
			"repository error lost: %v",
			err,
		)
	}
}
