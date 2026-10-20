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

// CreateCandidate stores a raw LLM question/answer pair. The tips feature was
// retired (ADR-075); the table keeps the learner Q&A history.
func (r *TipRepository) CreateCandidate(ctx context.Context, candidate *model.TipCandidate) error {
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO tip_candidates (
			user_id, username, language, proficiency_level, question, answer, source_model
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at
	`,
		candidate.UserID,
		candidate.Username,
		candidate.Language,
		candidate.ProficiencyLevel,
		candidate.Question,
		candidate.Answer,
		candidate.SourceModel,
	).Scan(&candidate.ID, &candidate.CreatedAt)
	if err != nil {
		return fmt.Errorf("TipRepository.CreateCandidate user_id=%d language=%s level=%s: %w",
			candidate.UserID, candidate.Language, candidate.ProficiencyLevel, err)
	}
	return nil
}
