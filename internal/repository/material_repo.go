package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/lsj/copylingo/internal/model"
)

type MaterialRepository struct {
	db *sqlx.DB
}

func NewMaterialRepository(db *sqlx.DB) *MaterialRepository {
	return &MaterialRepository{db: db}
}

// UpsertBatch inserts or refreshes materials identified by their stable material key.
func (r *MaterialRepository) UpsertBatch(ctx context.Context, materials []*model.Material) error {
	if len(materials) == 0 {
		return nil
	}

	query, args := buildMaterialBatchUpsertQuery(materials)
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("MaterialRepository.UpsertBatch count=%d: %w", len(materials), err)
	}
	return nil
}

func buildMaterialBatchUpsertQuery(materials []*model.Material) (string, []any) {
	const columnCount = 8

	var query strings.Builder
	query.WriteString(`
		INSERT INTO materials (
			material_key, content_id, category, language,
			proficiency_level, title, payload, difficulty
		)
		VALUES
	`)

	args := make([]any, 0, len(materials)*columnCount)
	for i, material := range materials {
		if i > 0 {
			query.WriteString(",")
		}

		base := i * columnCount
		query.WriteString(fmt.Sprintf(
			"($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
			base+1, base+2, base+3, base+4, base+5, base+6, base+7, base+8,
		))
		args = append(args,
			material.MaterialKey,
			material.ContentID,
			material.Category,
			material.Language,
			material.ProficiencyLevel,
			material.Title,
			material.Payload,
			material.Difficulty,
		)
	}

	query.WriteString(`
		ON CONFLICT (material_key) DO UPDATE SET
			content_id = EXCLUDED.content_id,
			category = EXCLUDED.category,
			language = EXCLUDED.language,
			proficiency_level = EXCLUDED.proficiency_level,
			title = EXCLUDED.title,
			payload = EXCLUDED.payload,
			difficulty = EXCLUDED.difficulty
	`)

	return query.String(), args
}
