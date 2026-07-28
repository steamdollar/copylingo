package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"

	"github.com/lsj/copylingo/internal/model"
)

type MaterialRepository struct {
	db *sqlx.DB
}

func NewMaterialRepository(db *sqlx.DB) *MaterialRepository {
	return &MaterialRepository{db: db}
}

// GetByMaterialKeys returns materials whose stable material keys are included in keys.
func (r *MaterialRepository) GetByMaterialKeys(ctx context.Context, keys []string) ([]model.Material, error) {
	if len(keys) == 0 {
		return nil, nil
	}

	var materials []model.Material
	if err := r.db.SelectContext(ctx, &materials, `
		SELECT *
		FROM materials
		WHERE material_key = ANY($1)
		ORDER BY material_key
	`, pq.Array(keys)); err != nil {
		return nil, fmt.Errorf("MaterialRepository.GetByMaterialKeys count=%d: %w", len(keys), err)
	}
	return materials, nil
}

// GetForStudySession returns level-matched due or new vocabulary or grammar materials for a user.
func (r *MaterialRepository) GetForStudySession(
	ctx context.Context,
	userID int64,
	language, level string,
	limit int,
) ([]model.Material, error) {
	if limit <= 0 {
		return nil, nil
	}

	var materials []model.Material
	if err := r.db.SelectContext(ctx, &materials, studySessionMaterialsQuery,
		userID, language, level, pq.Array(studySessionMaterialCategories), limit); err != nil {
		return nil, fmt.Errorf("MaterialRepository.GetForStudySession user_id=%d language=%s level=%s limit=%d: %w",
			userID, language, level, limit, err)
	}
	return materials, nil
}

// studySessionMaterialCategories are the material categories a study session
// draws from. Reading is additionally capped to one card per session in the
// query below (ADR-036) — passages take longer to read than word/grammar cards.
var studySessionMaterialCategories = []string{
	string(model.MaterialCategoryVocabulary),
	string(model.MaterialCategoryGrammar),
	string(model.MaterialCategoryReading),
}

const studySessionMaterialsQuery = `
		WITH candidates AS (
			SELECT
				m.*,
				ROW_NUMBER() OVER (
					PARTITION BY m.category
					ORDER BY
						CASE WHEN ump.next_review_at IS NULL THEN 1 ELSE 0 END ASC,
						ump.next_review_at ASC NULLS LAST,
						m.difficulty ASC,
						m.id ASC
				) AS category_rank,
				CASE m.category
					WHEN 'vocabulary' THEN 0
					WHEN 'grammar' THEN 1
					WHEN 'reading' THEN 2
					ELSE 3
				END AS category_order
			FROM materials m
			LEFT JOIN user_material_progress ump
			ON ump.material_id = m.id
				AND ump.user_id = $1
			WHERE m.language = $2
			AND m.proficiency_level = $3
			AND m.category = ANY($4)
			AND (ump.material_id IS NULL OR ump.next_review_at <= NOW())
			AND NOT EXISTS (
				SELECT 1
				FROM session_materials sm
				JOIN sessions s ON s.id = sm.session_id
				WHERE s.user_id = $1
					AND sm.material_id = m.id
					AND s.mode = 'study'
					AND s.status IN ('pending', 'in_progress')
			)
		)
		SELECT
			id,
			material_key,
			content_id,
			category,
			language,
			proficiency_level,
			title,
			payload,
			difficulty,
			created_at
		FROM candidates
		WHERE category <> 'reading' OR category_rank <= 1
		ORDER BY category_rank ASC, category_order ASC
		LIMIT $5
	`

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
