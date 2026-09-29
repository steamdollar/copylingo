package service

import (
	"context"

	"github.com/lsj/copylingo/internal/model"
)

// TipRepo is the tips-table boundary: active tips, curated candidates and
// generated tips.
type TipRepo interface {
	ListActive(
		ctx context.Context,
		language,
		level string,
		limit int,
	) ([]model.Tip, error)
	CreateCandidate(
		ctx context.Context,
		candidate *model.TipCandidate,
	) error
	tipGeneratorRepo
}

// TipService is the Tier1 entry for tips: listing, candidate storage and
// LLM-backed bucket top-up (ADR-059 §8.6).
type TipService struct {
	repo      TipRepo
	generator *TipGenerator
}

// NewTipService wires tip storage and generation. A nil generatorLLM keeps
// top-up unavailable (ErrAIConfigMissing) while listing and candidates work.
func NewTipService(
	repo TipRepo,
	generatorLLM tipGeneratorLLM,
	sourceModel string,
) *TipService {
	return &TipService{
		repo: repo,
		generator: NewTipGenerator(
			repo,
			generatorLLM,
			sourceModel,
		),
	}
}

func (s *TipService) ListActive(
	ctx context.Context,
	language,
	level string,
	limit int,
) ([]model.Tip, error) {
	return s.repo.ListActive(
		ctx,
		language,
		level,
		limit,
	)
}

func (s *TipService) CreateCandidate(
	ctx context.Context,
	candidate *model.TipCandidate,
) error {
	return s.repo.CreateCandidate(
		ctx,
		candidate,
	)
}

// TopUpBucket fills the (language, level) active-tip bucket toward TipBucketTarget.
func (s *TipService) TopUpBucket(
	ctx context.Context,
	language,
	level string,
) error {
	return s.generator.TopUpBucket(
		ctx,
		language,
		level,
	)
}
