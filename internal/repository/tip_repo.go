package repository

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/lsj/copylingo/internal/model"
)

type TipRepository struct {
	db *sqlx.DB
}

func NewTipRepository(db *sqlx.DB) *TipRepository {
	return &TipRepository{db: db}
}

// Create inserts a single tip and populates tip.ID.
func (r *TipRepository) Create(ctx context.Context, tip *model.Tip) error {
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO tips (language, proficiency_level, category, body, source_model, source_prompt_ver, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at
	`,
		tip.Language, tip.ProficiencyLevel, tip.Category, tip.Body,
		tip.SourceModel, tip.SourcePromptVer, tip.IsActive,
	).Scan(&tip.ID, &tip.CreatedAt)
	if err != nil {
		return fmt.Errorf("TipRepository.Create language=%s level=%s category=%s: %w",
			tip.Language, tip.ProficiencyLevel, tip.Category, err)
	}
	return nil
}

// ListActive returns all active tips for the given (language, level), most recent first.
// Used by the Mini App tip endpoint; clients shuffle/rotate.
func (r *TipRepository) ListActive(ctx context.Context, language, level string, limit int) ([]model.Tip, error) {
	var tips []model.Tip
	err := r.db.SelectContext(ctx, &tips, `
		SELECT * FROM tips
		WHERE language = $1 AND proficiency_level = $2 AND is_active = TRUE
		ORDER BY created_at DESC
		LIMIT $3
	`, language, level, limit)
	if err != nil {
		return nil, fmt.Errorf("TipRepository.ListActive language=%s level=%s: %w", language, level, err)
	}
	return tips, nil
}

// CountActive returns the number of active tips for the given (language, level).
// Used by the scheduler to decide whether to top up the bucket toward the 50 target.
func (r *TipRepository) CountActive(ctx context.Context, language, level string) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `
		SELECT COUNT(*) FROM tips
		WHERE language = $1 AND proficiency_level = $2 AND is_active = TRUE
	`, language, level)
	if err != nil {
		return 0, fmt.Errorf("TipRepository.CountActive language=%s level=%s: %w", language, level, err)
	}
	return count, nil
}
