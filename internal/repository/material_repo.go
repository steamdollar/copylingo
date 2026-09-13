package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"

	"github.com/lsj/copylingo/internal/model"
)

type materialDB interface {
	SelectContext(context.Context, any, string, ...any) error
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

type MaterialRepository struct {
	db materialDB
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

// GetForStudySession returns materials according to the per-category new and
// review quotas in plan. New materials prefer the current level, while due
// reviews remain ordered by oldest due time.
func (r *MaterialRepository) GetForStudySession(
	ctx context.Context,
	userID int64,
	language string,
	level string,
	levels []string,
	plan model.StudySessionPlan,
) ([]model.Material, error) {
	if len(plan.Quotas) == 0 {
		return nil, nil
	}
	if err := validateStudySessionPlan(plan); err != nil {
		return nil, fmt.Errorf("MaterialRepository.GetForStudySession invalid plan: %w", err)
	}
	limit := plan.TotalMaterialCount()
	if limit == 0 {
		return nil, nil
	}
	quotaJSON, err := json.Marshal(plan.Quotas)
	if err != nil {
		return nil, fmt.Errorf("MaterialRepository.GetForStudySession marshal plan: %w", err)
	}

	var materials []model.Material
	if err := r.db.SelectContext(ctx, &materials, studySessionMaterialsQuery,
		userID, language, level, pq.Array(levels), quotaJSON, limit); err != nil {
		return nil, fmt.Errorf("MaterialRepository.GetForStudySession user_id=%d language=%s level=%s limit=%d: %w",
			userID, language, level, limit, err)
	}
	return materials, nil
}

func validateStudySessionPlan(plan model.StudySessionPlan) error {
	seen := make(map[model.MaterialCategory]struct{}, len(plan.Quotas))
	for i, quota := range plan.Quotas {
		if quota.NewCount < 0 || quota.ReviewCount < 0 {
			return fmt.Errorf("quota index=%d category=%s has negative count", i, quota.Category)
		}
		switch quota.Category {
		case model.MaterialCategoryVocabulary, model.MaterialCategoryGrammar, model.MaterialCategoryReading:
		default:
			return fmt.Errorf("quota index=%d has unsupported category=%s", i, quota.Category)
		}
		if _, ok := seen[quota.Category]; ok {
			return fmt.Errorf("duplicate category=%s", quota.Category)
		}
		seen[quota.Category] = struct{}{}
	}
	return nil
}

const studySessionMaterialsQuery = `
	WITH quotas AS (
		SELECT
			q.category,
			q.new_count,
			q.review_count,
			q.new_count + q.review_count AS category_total
		FROM jsonb_to_recordset($5::jsonb) AS q(
			category text,
			new_count integer,
			review_count integer
		)
	),
	material_pool AS (
		SELECT
			m.*,
			ump.material_id AS progress_material_id,
			ump.next_review_at,
			CASE WHEN ump.material_id IS NULL THEN 'new' ELSE 'review' END AS bucket,
			CASE WHEN m.proficiency_level = $3 THEN 0 ELSE 1 END AS level_rank,
			RANDOM() AS random_order
		FROM materials m
		JOIN quotas q ON q.category = m.category
		LEFT JOIN user_material_progress ump
			ON ump.material_id = m.id
			AND ump.user_id = $1
		WHERE m.language = $2
			AND m.proficiency_level = ANY($4)
			AND (
				ump.material_id IS NULL
				OR ump.next_review_at <= NOW()
			)
			AND NOT EXISTS (
				SELECT 1
				FROM session_materials sm
				JOIN sessions s ON s.id = sm.session_id
				WHERE s.user_id = $1
					AND sm.material_id = m.id
					AND s.mode = 'study'
					AND s.status IN ('pending', 'in_progress')
			)
	),
	ranked AS (
		SELECT
			mp.*,
			ROW_NUMBER() OVER (
				PARTITION BY mp.category, mp.bucket
				ORDER BY
					CASE WHEN mp.bucket = 'new' THEN mp.level_rank END ASC NULLS LAST,
					CASE WHEN mp.bucket = 'new' THEN mp.difficulty END ASC NULLS LAST,
					CASE WHEN mp.bucket = 'new' THEN mp.random_order END ASC NULLS LAST,
					CASE WHEN mp.bucket = 'review' THEN mp.next_review_at END ASC NULLS LAST,
					CASE WHEN mp.bucket = 'review' THEN mp.id END ASC
			) AS bucket_rank
		FROM material_pool mp
	),
	primary_selected AS (
		SELECT r.*
		FROM ranked r
		JOIN quotas q ON q.category = r.category
		WHERE r.bucket = 'new' AND r.bucket_rank <= q.new_count
		UNION ALL
		SELECT r.*
		FROM ranked r
		JOIN quotas q ON q.category = r.category
		WHERE r.bucket = 'review' AND r.bucket_rank <= q.review_count
	),
	primary_counts AS (
		SELECT
			q.category,
			q.new_count,
			q.review_count,
			q.category_total,
			COUNT(p.id) AS primary_count,
			COUNT(p.id) FILTER (WHERE p.bucket = 'new') AS new_selected,
			COUNT(p.id) FILTER (WHERE p.bucket = 'review') AS review_selected
		FROM quotas q
		LEFT JOIN primary_selected p ON p.category = q.category
		GROUP BY q.category, q.new_count, q.review_count, q.category_total
	),
	remaining_due AS (
		SELECT
			r.*,
			pc.new_count - pc.new_selected AS new_missing,
			pc.review_count - pc.review_selected AS review_missing,
			pc.category_total,
			pc.primary_count,
			ROW_NUMBER() OVER (
				PARTITION BY r.category
				ORDER BY r.next_review_at ASC, r.id ASC
			) AS due_category_rank
		FROM ranked r
		JOIN primary_counts pc ON pc.category = r.category
		WHERE r.bucket = 'review'
			AND r.bucket_rank > pc.review_selected
	),
	same_category_due AS (
		SELECT rd.*
		FROM remaining_due rd
		WHERE rd.new_missing > 0
			AND (rd.category <> 'reading' OR rd.primary_count < rd.category_total)
			AND rd.due_category_rank <= rd.new_missing
	),
	other_due AS (
		SELECT
			rd.*,
			ROW_NUMBER() OVER (
				ORDER BY rd.next_review_at ASC, rd.id ASC
			) AS fallback_rank
		FROM remaining_due rd
		WHERE rd.category IN ('vocabulary', 'grammar')
			AND NOT EXISTS (
				SELECT 1
				FROM same_category_due sd
				WHERE sd.id = rd.id
			)
	),
	selected_other_due AS (
		SELECT od.*
		FROM other_due od
		WHERE od.fallback_rank <= GREATEST(
			$6
			- (SELECT COUNT(*) FROM primary_selected)
			- (SELECT COUNT(*) FROM same_category_due),
			0
		)
	),
	selected AS (
		SELECT
			id, material_key, content_id, category, language,
			proficiency_level, title, payload, difficulty, created_at,
			progress_material_id, next_review_at, bucket, level_rank, random_order
		FROM primary_selected
		UNION ALL
		SELECT
			id, material_key, content_id, category, language,
			proficiency_level, title, payload, difficulty, created_at,
			progress_material_id, next_review_at, bucket, level_rank, random_order
		FROM same_category_due
		UNION ALL
		SELECT
			id, material_key, content_id, category, language,
			proficiency_level, title, payload, difficulty, created_at,
			progress_material_id, next_review_at, bucket, level_rank, random_order
		FROM selected_other_due
	),
	ordered AS (
		SELECT
			s.*,
			ROW_NUMBER() OVER (
				PARTITION BY s.category
				ORDER BY
					CASE WHEN s.bucket = 'new' THEN 0 ELSE 1 END,
					CASE WHEN s.bucket = 'new' THEN s.level_rank END ASC NULLS LAST,
					CASE WHEN s.bucket = 'new' THEN s.difficulty END ASC NULLS LAST,
					CASE WHEN s.bucket = 'new' THEN s.random_order END ASC NULLS LAST,
					CASE WHEN s.bucket = 'review' THEN s.next_review_at END ASC NULLS LAST,
					CASE WHEN s.bucket = 'review' THEN s.id END ASC
			) AS category_rank,
			CASE s.category
				WHEN 'vocabulary' THEN 0
				WHEN 'grammar' THEN 1
				WHEN 'reading' THEN 2
				ELSE 3
			END AS category_order
		FROM selected s
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
	FROM ordered
	ORDER BY category_rank ASC, category_order ASC, id ASC
	LIMIT $6
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
