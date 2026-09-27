package service

import (
	"context"
	"fmt"

	"github.com/lsj/copylingo/internal/model"
)

const MaterialPreferencePageSize = 8

type materialPreferenceRepo interface {
	Get(ctx context.Context, userID int64, materialID int) (*model.MaterialPreference, error)
	Set(ctx context.Context, userID int64, materialID int, mode model.MaterialReviewMode) error
	List(ctx context.Context, userID int64, offset, limit int) ([]model.MaterialPreference, error)
}

type MaterialPreferenceService struct {
	repo materialPreferenceRepo
}

func NewMaterialPreferenceService(repo materialPreferenceRepo) *MaterialPreferenceService {
	return &MaterialPreferenceService{repo: repo}
}

func (s *MaterialPreferenceService) Get(
	ctx context.Context,
	userID int64,
	materialID int,
) (*model.MaterialPreference, error) {
	if userID <= 0 || materialID <= 0 {
		return nil, fmt.Errorf("MaterialPreferenceService.Get: invalid user or material ID")
	}
	return s.repo.Get(ctx, userID, materialID)
}

func (s *MaterialPreferenceService) Set(
	ctx context.Context,
	userID int64,
	materialID int,
	mode model.MaterialReviewMode,
) error {
	if userID <= 0 || materialID <= 0 || !mode.Valid() {
		return fmt.Errorf("MaterialPreferenceService.Set: invalid user, material or review mode")
	}
	return s.repo.Set(ctx, userID, materialID, mode)
}

// List fetches one extra row to determine pagination without a separate count.
func (s *MaterialPreferenceService) List(
	ctx context.Context,
	userID int64,
	page int,
) ([]model.MaterialPreference, bool, error) {
	maxPage := (int(^uint(0)>>1) - MaterialPreferencePageSize) / MaterialPreferencePageSize
	if userID <= 0 || page < 0 || page > maxPage {
		return nil, false, fmt.Errorf("MaterialPreferenceService.List: invalid user or page")
	}
	items, err := s.repo.List(ctx, userID, page*MaterialPreferencePageSize, MaterialPreferencePageSize+1)
	if err != nil {
		return nil, false, err
	}
	hasNext := len(items) > MaterialPreferencePageSize
	if hasNext {
		items = items[:MaterialPreferencePageSize]
	}
	return items, hasNext, nil
}
