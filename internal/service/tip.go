package service

import (
	"context"

	"github.com/lsj/copylingo/internal/model"
)

type tipRepository interface {
	ListActive(ctx context.Context, language, level string, limit int) ([]model.Tip, error)
	CreateCandidate(ctx context.Context, candidate *model.TipCandidate) error
}

type TipService struct {
	repo tipRepository
}

func NewTipService(repo tipRepository) *TipService {
	return &TipService{repo: repo}
}

func (s *TipService) ListActive(ctx context.Context, language, level string, limit int) ([]model.Tip, error) {
	return s.repo.ListActive(ctx, language, level, limit)
}

func (s *TipService) CreateCandidate(ctx context.Context, candidate *model.TipCandidate) error {
	return s.repo.CreateCandidate(ctx, candidate)
}
