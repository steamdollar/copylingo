package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/lsj/copylingo/internal/model"
)

// ActiveSessionRepository loads and flushes Redis-backed question session state.
type ActiveSessionRepository struct {
	db *sqlx.DB
}

func NewActiveSessionRepository(db *sqlx.DB) *ActiveSessionRepository {
	return &ActiveSessionRepository{db: db}
}

type questionSessionWithStateRow struct {
	SessionID              int                    `db:"session_id"`
	UserID                 int64                  `db:"user_id"`
	SessionType            model.SessionType      `db:"session_type"`
	SessionMode            model.SessionMode      `db:"session_mode"`
	SessionStatus          model.SessionStatus    `db:"session_status"`
	TotalQuestions         int                    `db:"total_questions"`
	CorrectCount           int                    `db:"correct_count"`
	StartedAt              *time.Time             `db:"started_at"`
	CompletedAt            *time.Time             `db:"completed_at"`
	SessionCreatedAt       time.Time              `db:"session_created_at"`
	SessionQuestionID      int                    `db:"session_question_id"`
	QuestionOrder          int                    `db:"question_order"`
	IsReview               bool                   `db:"is_review"`
	UserAnswer             *string                `db:"user_answer"`
	IsCorrect              *bool                  `db:"is_correct"`
	QuestionID             int                    `db:"question_id"`
	ContentID              *int                   `db:"content_id"`
	MaterialID             *int                   `db:"material_id"`
	QuestionType           model.QuestionType     `db:"question_type"`
	Language               string                 `db:"language"`
	ProficiencyLevel       string                 `db:"proficiency_level"`
	Category               model.QuestionCategory `db:"category"`
	Prompt                 string                 `db:"prompt"`
	Options                json.RawMessage        `db:"options"`
	CorrectAnswer          string                 `db:"correct_answer"`
	Explanation            string                 `db:"explanation"`
	AudioPath              *string                `db:"audio_path"`
	Difficulty             int                    `db:"difficulty"`
	ProgressEaseFactor     float64                `db:"progress_ease_factor"`
	ProgressIntervalDays   int                    `db:"progress_interval_days"`
	ProgressRepetitions    int                    `db:"progress_repetitions"`
	ProgressNextReviewAt   *time.Time             `db:"progress_next_review_at"`
	ProgressLastReviewedAt *time.Time             `db:"progress_last_reviewed_at"`
	ProgressTimesServed    int                    `db:"progress_times_served"`
	ProgressTimesCorrect   int                    `db:"progress_times_correct"`
	QuestionCreatedAt      time.Time              `db:"question_created_at"`
}

// LoadQuestionSessionWithStateBySessionID loads the full ordered question session state in one DB round-trip.
func (r *ActiveSessionRepository) LoadQuestionSessionWithStateBySessionID(
	ctx context.Context,
	sessionID int,
) (*model.ActiveSessionState, error) {
	var rows []questionSessionWithStateRow
	if err := r.db.SelectContext(ctx, &rows, `
		SELECT
			s.id AS session_id,
			s.user_id,
			s.type AS session_type,
			s.mode AS session_mode,
			s.status AS session_status,
			s.total_questions,
			s.correct_count,
			s.started_at,
			s.completed_at,
			s.created_at AS session_created_at,
			sq.id AS session_question_id,
			sq.question_order,
			sq.is_review,
			sq.user_answer,
			sq.is_correct,
			q.id AS question_id,
			q.content_id,
			q.material_id,
			q.type AS question_type,
			q.language,
			q.proficiency_level,
			q.category,
			q.prompt,
			COALESCE(q.options, 'null'::jsonb) AS options,
			q.correct_answer,
			q.explanation,
			q.audio_path,
			q.difficulty,
			COALESCE(uqp.ease_factor, 2.5) AS progress_ease_factor,
			COALESCE(uqp.interval_days, 0) AS progress_interval_days,
			COALESCE(uqp.repetitions, 0) AS progress_repetitions,
			uqp.next_review_at AS progress_next_review_at,
			uqp.last_reviewed_at AS progress_last_reviewed_at,
			COALESCE(uqp.times_served, 0) AS progress_times_served,
			COALESCE(uqp.times_correct, 0) AS progress_times_correct,
			q.created_at AS question_created_at
		FROM sessions s
		JOIN session_questions sq ON sq.session_id = s.id
		JOIN questions q ON q.id = sq.question_id
		LEFT JOIN user_question_progress uqp
			ON uqp.user_id = s.user_id AND uqp.question_id = q.id
		WHERE s.id = $1
		ORDER BY sq.question_order
	`, sessionID); err != nil {
		return nil, fmt.Errorf(
			"ActiveSessionRepository.LoadQuestionSessionWithStateBySessionID session_id=%d: %w",
			sessionID,
			err,
		)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf(
			"ActiveSessionRepository.LoadQuestionSessionWithStateBySessionID session_id=%d: %w",
			sessionID,
			sql.ErrNoRows,
		)
	}

	first := rows[0]
	state := &model.ActiveSessionState{
		Version: model.ActiveSessionStateVersion,
		Session: model.Session{
			ID:             first.SessionID,
			UserID:         first.UserID,
			Type:           first.SessionType,
			Mode:           first.SessionMode,
			Status:         first.SessionStatus,
			TotalQuestions: first.TotalQuestions,
			CorrectCount:   first.CorrectCount,
			StartedAt:      first.StartedAt,
			CompletedAt:    first.CompletedAt,
			CreatedAt:      first.SessionCreatedAt,
		},
		Items:        make([]model.ActiveSessionQuestion, 0, len(rows)),
		UpdatedAt:    time.Now(),
		CurrentIndex: 0,
	}

	for _, row := range rows {
		state.Items = append(state.Items, model.ActiveSessionQuestion{
			SessionQuestion: model.SessionQuestion{
				ID:            row.SessionQuestionID,
				SessionID:     row.SessionID,
				QuestionID:    row.QuestionID,
				QuestionOrder: row.QuestionOrder,
				IsReview:      row.IsReview,
				UserAnswer:    row.UserAnswer,
				IsCorrect:     row.IsCorrect,
			},
			Question: model.Question{
				ID:               row.QuestionID,
				ContentID:        row.ContentID,
				MaterialID:       row.MaterialID,
				Type:             row.QuestionType,
				Language:         row.Language,
				ProficiencyLevel: row.ProficiencyLevel,
				Category:         row.Category,
				Prompt:           row.Prompt,
				Options:          row.Options,
				CorrectAnswer:    row.CorrectAnswer,
				Explanation:      row.Explanation,
				AudioPath:        row.AudioPath,
				Difficulty:       row.Difficulty,
				CreatedAt:        row.QuestionCreatedAt,
			},
			Progress: model.UserQuestionProgress{
				UserID:         row.UserID,
				QuestionID:     row.QuestionID,
				EaseFactor:     row.ProgressEaseFactor,
				IntervalDays:   row.ProgressIntervalDays,
				Repetitions:    row.ProgressRepetitions,
				NextReviewAt:   row.ProgressNextReviewAt,
				LastReviewedAt: row.ProgressLastReviewedAt,
				TimesServed:    row.ProgressTimesServed,
				TimesCorrect:   row.ProgressTimesCorrect,
			},
		})
	}
	state.RecountAnswered()

	return state, nil
}

// FlushActiveSession persists the question session state in a single DB transaction.
func (r *ActiveSessionRepository) FlushActiveSession(ctx context.Context, state *model.ActiveSessionState) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ActiveSessionRepository.FlushActiveSession begin session_id=%d: %w", state.Session.ID, err)
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	flushed, err := markSessionCompleted(ctx, tx, state)
	if err != nil {
		return err
	}
	if flushed {
		if err := flushSessionQuestions(ctx, tx, state.Items); err != nil {
			return err
		}
		if err := flushQuestionProgress(ctx, tx, state.Items); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("ActiveSessionRepository.FlushActiveSession commit session_id=%d: %w", state.Session.ID, err)
	}
	committed = true
	return nil
}

func markSessionCompleted(ctx context.Context, tx *sqlx.Tx, state *model.ActiveSessionState) (bool, error) {
	correctCount := state.CorrectCount()
	res, err := tx.ExecContext(ctx, `
		UPDATE sessions
		SET status = $2, correct_count = $3, completed_at = NOW()
		WHERE id = $1 AND status <> $2
	`, state.Session.ID, model.SessionCompleted, correctCount)
	if err != nil {
		return false, fmt.Errorf(
			"ActiveSessionRepository.markSessionCompleted session_id=%d: %w",
			state.Session.ID,
			err,
		)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf(
			"ActiveSessionRepository.markSessionCompleted rows session_id=%d: %w",
			state.Session.ID,
			err,
		)
	}
	return rows > 0, nil
}

func flushSessionQuestions(ctx context.Context, tx *sqlx.Tx, items []model.ActiveSessionQuestion) error {
	if len(items) == 0 {
		return nil
	}

	var values strings.Builder
	args := make([]any, 0, len(items)*3)
	for i, item := range items {
		if i > 0 {
			values.WriteString(",")
		}
		base := i * 3
		values.WriteString(fmt.Sprintf("($%d,$%d,$%d)", base+1, base+2, base+3))
		args = append(args, item.SessionQuestion.ID, item.SessionQuestion.UserAnswer, item.SessionQuestion.IsCorrect)
	}

	query := fmt.Sprintf(`
		UPDATE session_questions AS sq
		SET user_answer = v.user_answer::text,
			is_correct = v.is_correct::boolean
		FROM (VALUES %s) AS v(id, user_answer, is_correct)
		WHERE sq.id = v.id::int
	`, values.String())
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("ActiveSessionRepository.flushSessionQuestions count=%d: %w", len(items), err)
	}
	return nil
}

type questionProgressFlushRow struct {
	Progress     model.UserQuestionProgress
	ServedDelta  int
	CorrectDelta int
}

func flushQuestionProgress(ctx context.Context, tx *sqlx.Tx, items []model.ActiveSessionQuestion) error {
	query, args, count := buildQuestionProgressUpsert(items)
	if count == 0 {
		return nil
	}
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("ActiveSessionRepository.flushQuestionProgress count=%d: %w", count, err)
	}
	return nil
}

func buildQuestionProgressUpsert(items []model.ActiveSessionQuestion) (string, []any, int) {
	rowsByID := make(map[int]*questionProgressFlushRow)
	ids := make([]int, 0, len(items))
	for _, item := range items {
		if item.SessionQuestion.IsCorrect == nil {
			continue
		}

		row, ok := rowsByID[item.Question.ID]
		if !ok {
			row = &questionProgressFlushRow{Progress: item.Progress}
			rowsByID[item.Question.ID] = row
			ids = append(ids, item.Question.ID)
		}
		row.Progress = item.Progress
		row.ServedDelta++
		if *item.SessionQuestion.IsCorrect {
			row.CorrectDelta++
		}
	}
	if len(ids) == 0 {
		return "", nil, 0
	}

	var values strings.Builder
	args := make([]any, 0, len(ids)*9)
	for i, id := range ids {
		if i > 0 {
			values.WriteString(",")
		}
		base := i * 9
		values.WriteString(fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)",
			base+1, base+2, base+3, base+4, base+5, base+6, base+7, base+8, base+9))

		row := rowsByID[id]
		progress := row.Progress
		args = append(args,
			progress.UserID,
			progress.QuestionID,
			row.ServedDelta,
			row.CorrectDelta,
			progress.EaseFactor,
			progress.IntervalDays,
			progress.Repetitions,
			progress.NextReviewAt,
			progress.LastReviewedAt,
		)
	}

	query := fmt.Sprintf(`
		INSERT INTO user_question_progress (
			user_id, question_id, times_served, times_correct, ease_factor,
			interval_days, repetitions, next_review_at, last_reviewed_at
		)
		VALUES %s
		ON CONFLICT (user_id, question_id) DO UPDATE SET
			times_served = user_question_progress.times_served + EXCLUDED.times_served,
			times_correct = user_question_progress.times_correct + EXCLUDED.times_correct,
			ease_factor = EXCLUDED.ease_factor,
			interval_days = EXCLUDED.interval_days,
			repetitions = EXCLUDED.repetitions,
			next_review_at = EXCLUDED.next_review_at,
			last_reviewed_at = EXCLUDED.last_reviewed_at,
			updated_at = NOW()
	`, values.String())
	return query, args, len(ids)
}
