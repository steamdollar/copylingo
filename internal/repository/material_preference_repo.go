package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/lsj/copylingo/internal/model"
)

type materialPreferenceDB interface {
	GetContext(context.Context, any, string, ...any) error
	SelectContext(context.Context, any, string, ...any) error
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

type MaterialPreferenceRepository struct {
	db materialPreferenceDB
}

func NewMaterialPreferenceRepository(db *sqlx.DB) *MaterialPreferenceRepository {
	return &MaterialPreferenceRepository{db: db}
}

func (r *MaterialPreferenceRepository) Get(
	ctx context.Context,
	userID int64,
	materialID int,
) (*model.MaterialPreference, error) {
	var preference model.MaterialPreference
	if err := r.db.GetContext(ctx, &preference, `
		SELECT p.*, m.title AS material_title
		FROM user_material_preferences p
		JOIN materials m ON m.id = p.material_id
		WHERE p.user_id = $1 AND p.material_id = $2
	`, userID, materialID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &model.MaterialPreference{
				UserID: userID, MaterialID: materialID, ReviewMode: model.MaterialReviewNormal,
				CheckIntervalDays: model.InitialMaintenanceDays,
			}, nil
		}
		return nil, fmt.Errorf(
			"MaterialPreferenceRepository.Get user_id=%d material_id=%d: %w",
			userID,
			materialID,
			err,
		)
	}
	return &preference, nil
}

// Set changes only the user's preference; catalog and learning history stay intact.
// Repeating the current mode must not postpone a maintenance check.
func (r *MaterialPreferenceRepository) Set(
	ctx context.Context,
	userID int64,
	materialID int,
	mode model.MaterialReviewMode,
) error {
	var err error
	switch mode {
	case model.MaterialReviewNormal:
		_, err = r.db.ExecContext(
			ctx,
			`DELETE FROM user_material_preferences WHERE user_id = $1 AND material_id = $2`,
			userID,
			materialID,
		)
	case model.MaterialReviewMaintenance, model.MaterialReviewExcluded:
		_, err = r.db.ExecContext(ctx, `
			INSERT INTO user_material_preferences (user_id, material_id, review_mode, next_check_at, check_interval_days)
			VALUES ($1, $2, $3, CASE WHEN $3 = 'maintenance' THEN NOW() + $4 * INTERVAL '1 day' END, $4)
			ON CONFLICT (user_id, material_id) DO UPDATE SET
				review_mode = EXCLUDED.review_mode,
				next_check_at = EXCLUDED.next_check_at,
				check_interval_days = EXCLUDED.check_interval_days,
				updated_at = NOW()
			WHERE user_material_preferences.review_mode <> EXCLUDED.review_mode
		`, userID, materialID, mode, model.InitialMaintenanceDays)
	default:
		return fmt.Errorf(
			"MaterialPreferenceRepository.Set user_id=%d material_id=%d: invalid review_mode=%q",
			userID,
			materialID,
			mode,
		)
	}
	if err != nil {
		return fmt.Errorf(
			"MaterialPreferenceRepository.Set user_id=%d material_id=%d mode=%s: %w",
			userID,
			materialID,
			mode,
			err,
		)
	}
	return nil
}

func (r *MaterialPreferenceRepository) List(
	ctx context.Context,
	userID int64,
	offset, limit int,
) ([]model.MaterialPreference, error) {
	var preferences []model.MaterialPreference
	if err := r.db.SelectContext(ctx, &preferences, `
		SELECT p.*, m.title AS material_title
		FROM user_material_preferences p
		JOIN materials m ON m.id = p.material_id
		WHERE p.user_id = $1
		ORDER BY p.material_id
		OFFSET $2 LIMIT $3
	`, userID, offset, limit); err != nil {
		return nil, fmt.Errorf(
			"MaterialPreferenceRepository.List user_id=%d offset=%d limit=%d: %w",
			userID,
			offset,
			limit,
			err,
		)
	}
	return preferences, nil
}

// A stable representative prevents multiple variants of one maintenance material
// from entering a session through separate category, new, and due queries. Due
// questions take precedence; ordinary questions precede optional kanji recall.
// Admission is shared with Study so unread passages and missing audio can fall
// back to a Study card instead of permanently blocking a maintenance check.
func maintenanceQuizCandidatesCTE(levelsPlaceholder string) string {
	return fmt.Sprintf(`maintenance_quiz_candidates AS (
	SELECT q.id, q.material_id, q.language, q.proficiency_level,
		ROW_NUMBER() OVER (
			PARTITION BY q.material_id
			ORDER BY CASE WHEN uqp.question_id IS NOT NULL THEN 0 ELSE 1 END,
				CASE WHEN q.item_type = 'vocab_kanji_recall' THEN 1 ELSE 0 END,
				uqp.next_review_at ASC NULLS LAST, q.id
		) AS candidate_rank
	FROM user_material_preferences preference
	JOIN questions q ON q.material_id = preference.material_id
	LEFT JOIN user_question_progress uqp ON uqp.user_id = $1 AND uqp.question_id = q.id
	LEFT JOIN user_material_progress ump ON ump.user_id = $1 AND ump.material_id = q.material_id AND ump.times_studied > 0
	WHERE preference.user_id = $1
		AND preference.review_mode = 'maintenance'
		AND preference.next_check_at <= NOW()
		AND q.language = $2 AND q.proficiency_level = ANY(%s)
		AND (uqp.question_id IS NULL OR uqp.next_review_at <= NOW())
		AND (q.category <> 'listening' OR q.audio_path IS NOT NULL)
		AND (q.category <> 'reading' OR ump.material_id IS NOT NULL)
)`, levelsPlaceholder)
}

const quizMaterialPreferenceGate = `
	AND (preference.material_id IS NULL OR (
		preference.review_mode = 'maintenance'
		AND preference.next_check_at <= NOW()
		AND q.id IN (SELECT id FROM maintenance_quiz_candidates WHERE candidate_rank = 1)
	))`
