package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/lsj/copylingo/internal/model"
)

// StudyActiveSessionRepository loads and flushes Redis-backed study session state.
type StudyActiveSessionRepository struct {
	db *sqlx.DB
}

func NewStudyActiveSessionRepository(db *sqlx.DB) *StudyActiveSessionRepository {
	return &StudyActiveSessionRepository{db: db}
}

type studyActiveSessionRow struct {
	SessionIDForParent int                 `db:"session_id_for_parent"`
	UserID             int64               `db:"user_id"`
	SessionType        model.SessionType   `db:"session_type"`
	Mode               model.SessionMode   `db:"mode"`
	Status             model.SessionStatus `db:"status"`
	TotalQuestions     int                 `db:"total_questions"`
	CorrectCount       int                 `db:"correct_count"`
	StartedAt          *time.Time          `db:"started_at"`
	CompletedAt        *time.Time          `db:"completed_at"`
	SessionCreatedAt   time.Time           `db:"session_created_at"`

	SessionMaterialID int        `db:"session_material_id"`
	SessionID         int        `db:"session_id"`
	MaterialID        int        `db:"material_id"`
	MaterialOrder     int        `db:"material_order"`
	StudiedAt         *time.Time `db:"studied_at"`
	RowCreatedAt      time.Time  `db:"row_created_at"`

	MaterialKey       string                 `db:"material_key"`
	ContentID         *int                   `db:"content_id"`
	Category          model.MaterialCategory `db:"category"`
	Language          string                 `db:"language"`
	ProficiencyLevel  string                 `db:"proficiency_level"`
	Title             string                 `db:"title"`
	Payload           []byte                 `db:"payload"`
	Difficulty        int                    `db:"difficulty"`
	MaterialCreatedAt time.Time              `db:"material_created_at"`
}

func (r *StudyActiveSessionRepository) LoadStudyActiveSession(
	ctx context.Context,
	sessionID int,
) (*model.StudyActiveSessionState, error) {
	var rows []studyActiveSessionRow
	if err := r.db.SelectContext(ctx,
		&rows,
		`
			SELECT
				s.id AS session_id_for_parent,
				s.user_id,
				s.type AS session_type,
				s.mode,
				s.status,
				s.total_questions,
				s.correct_count,
				s.started_at,
				s.completed_at,
				s.created_at AS session_created_at,
				sm.id AS session_material_id,
				sm.session_id,
				sm.material_id,
				sm.material_order,
				sm.studied_at,
				sm.created_at AS row_created_at,
				m.material_key,
				m.content_id,
				m.category,
				m.language,
				m.proficiency_level,
				m.title,
				m.payload,
				m.difficulty,
				m.created_at AS material_created_at
			FROM sessions s
			JOIN session_materials sm ON sm.session_id = s.id
			JOIN materials m ON m.id = sm.material_id
			WHERE s.id = $1
			  AND s.mode = $2
			ORDER BY sm.material_order
		`,
		sessionID,
		model.SessionModeStudy,
	); err != nil {
		return nil, fmt.Errorf("StudyActiveSessionRepository.LoadStudyActiveSession session_id=%d: %w", sessionID, err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf(
			"StudyActiveSessionRepository.LoadStudyActiveSession session_id=%d: %w",
			sessionID,
			sql.ErrNoRows,
		)
	}

	session := studySessionFromRow(rows[0])
	state := &model.StudyActiveSessionState{
		Version:      model.StudyActiveSessionStateVersion,
		Session:      session,
		Items:        make([]model.StudySessionMaterial, 0, len(rows)),
		UpdatedAt:    time.Now(),
		CurrentIndex: 0,
	}
	for _, row := range rows {
		state.Items = append(state.Items, studyActiveSessionMaterialFromRow(row))
	}
	state.RecountStudied()
	state.CaptureInitiallyStudied()
	state.CurrentIndex = state.NextUnstudiedIndex()
	return state, nil
}

func studySessionFromRow(row studyActiveSessionRow) model.Session {
	return model.Session{
		ID:             row.SessionIDForParent,
		UserID:         row.UserID,
		Type:           row.SessionType,
		Mode:           row.Mode,
		Status:         row.Status,
		TotalQuestions: row.TotalQuestions,
		CorrectCount:   row.CorrectCount,
		StartedAt:      row.StartedAt,
		CompletedAt:    row.CompletedAt,
		CreatedAt:      row.SessionCreatedAt,
	}
}

func (r *StudyActiveSessionRepository) FlushStudyActiveSession(
	ctx context.Context,
	state *model.StudyActiveSessionState,
) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf(
			"StudyActiveSessionRepository.FlushStudyActiveSession begin session_id=%d: %w",
			state.Session.ID,
			err,
		)
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	flushed, err := markStudySessionCompleted(ctx, tx, state)
	if err != nil {
		return err
	}
	if flushed {
		if err := flushStudySessionMaterials(ctx, tx, state.Items); err != nil {
			return err
		}
		if err := flushUserMaterialProgress(ctx, tx, state); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf(
			"StudyActiveSessionRepository.FlushStudyActiveSession commit session_id=%d: %w",
			state.Session.ID,
			err,
		)
	}
	committed = true
	return nil
}

func studyActiveSessionMaterialFromRow(row studyActiveSessionRow) model.StudySessionMaterial {
	return model.StudySessionMaterial{
		SessionMaterial: model.SessionMaterial{
			ID:            row.SessionMaterialID,
			SessionID:     row.SessionID,
			MaterialID:    row.MaterialID,
			MaterialOrder: row.MaterialOrder,
			StudiedAt:     row.StudiedAt,
			CreatedAt:     row.RowCreatedAt,
		},
		Material: model.Material{
			ID:               row.MaterialID,
			MaterialKey:      row.MaterialKey,
			ContentID:        row.ContentID,
			Category:         row.Category,
			Language:         row.Language,
			ProficiencyLevel: row.ProficiencyLevel,
			Title:            row.Title,
			Payload:          row.Payload,
			Difficulty:       row.Difficulty,
			CreatedAt:        row.MaterialCreatedAt,
		},
	}
}

func markStudySessionCompleted(ctx context.Context, tx *sqlx.Tx, state *model.StudyActiveSessionState) (bool, error) {
	res, err := tx.ExecContext(ctx, `
		UPDATE sessions
		SET status = $2, correct_count = 0, completed_at = NOW()
		WHERE id = $1 AND status <> $2
	`, state.Session.ID, model.SessionCompleted)
	if err != nil {
		return false, fmt.Errorf(
			"StudyActiveSessionRepository.markStudySessionCompleted session_id=%d: %w",
			state.Session.ID,
			err,
		)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf(
			"StudyActiveSessionRepository.markStudySessionCompleted rows session_id=%d: %w",
			state.Session.ID,
			err,
		)
	}
	return rows > 0, nil
}

func flushStudySessionMaterials(ctx context.Context, tx *sqlx.Tx, items []model.StudySessionMaterial) error {
	studied := make([]model.SessionMaterial, 0, len(items))
	for _, item := range items {
		if item.SessionMaterial.StudiedAt != nil {
			studied = append(studied, item.SessionMaterial)
		}
	}
	if len(studied) == 0 {
		return nil
	}

	var values strings.Builder
	args := make([]any, 0, len(studied)*2)
	for i, item := range studied {
		if i > 0 {
			values.WriteString(",")
		}
		base := i * 2
		values.WriteString(fmt.Sprintf("($%d,$%d)", base+1, base+2))
		args = append(args, item.ID, item.StudiedAt)
	}

	query := fmt.Sprintf(`
		UPDATE session_materials AS sm
		SET studied_at = COALESCE(sm.studied_at, v.studied_at::timestamptz)
		FROM (VALUES %s) AS v(id, studied_at)
		WHERE sm.id = v.id::int
	`, values.String())
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("StudyActiveSessionRepository.flushStudySessionMaterials count=%d: %w", len(studied), err)
	}
	return nil
}

func flushUserMaterialProgress(ctx context.Context, tx *sqlx.Tx, state *model.StudyActiveSessionState) error {
	materialIDs := state.NewlyStudiedMaterialIDs()
	if len(materialIDs) == 0 {
		return nil
	}

	var values strings.Builder
	args := make([]any, 0, len(materialIDs)+1)
	args = append(args, state.Session.UserID)
	for i, materialID := range materialIDs {
		if i > 0 {
			values.WriteString(",")
		}
		placeholder := i + 2
		values.WriteString(fmt.Sprintf("($%d)", placeholder))
		args = append(args, materialID)
	}

	query := fmt.Sprintf(`
		INSERT INTO user_material_progress (
			user_id, material_id, ease_factor, interval_days, repetitions,
			next_review_at, last_studied_at, times_studied
		)
		SELECT $1, v.material_id::int, 2.5, 1, 1, NOW() + INTERVAL '1 day', NOW(), 1
		FROM (VALUES %s) AS v(material_id)
		ON CONFLICT (user_id, material_id) DO UPDATE SET
			interval_days = CASE
				WHEN user_material_progress.repetitions = 0 THEN 1
				WHEN user_material_progress.repetitions = 1 THEN 6
				ELSE GREATEST(1, ROUND(user_material_progress.interval_days * user_material_progress.ease_factor)::int)
			END,
			repetitions = user_material_progress.repetitions + 1,
			next_review_at = NOW() + (
				CASE
					WHEN user_material_progress.repetitions = 0 THEN 1
					WHEN user_material_progress.repetitions = 1 THEN 6
					ELSE GREATEST(1, ROUND(user_material_progress.interval_days * user_material_progress.ease_factor)::int)
				END * INTERVAL '1 day'
			),
			last_studied_at = NOW(),
			times_studied = user_material_progress.times_studied + 1,
			updated_at = NOW()
	`, values.String())
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("StudyActiveSessionRepository.flushUserMaterialProgress count=%d: %w", len(materialIDs), err)
	}
	return nil
}
