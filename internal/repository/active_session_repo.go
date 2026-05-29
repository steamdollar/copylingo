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

// ActiveSessionRepository loads and flushes Redis-backed active session state.
type ActiveSessionRepository struct {
	db *sqlx.DB
}

func NewActiveSessionRepository(db *sqlx.DB) *ActiveSessionRepository {
	return &ActiveSessionRepository{db: db}
}

type activeSessionRow struct {
	SessionID         int                    `db:"session_id"`
	UserID            int64                  `db:"user_id"`
	SessionType       model.SessionType      `db:"session_type"`
	SessionStatus     model.SessionStatus    `db:"session_status"`
	TotalQuestions    int                    `db:"total_questions"`
	CorrectCount      int                    `db:"correct_count"`
	StartedAt         *time.Time             `db:"started_at"`
	CompletedAt       *time.Time             `db:"completed_at"`
	SessionCreatedAt  time.Time              `db:"session_created_at"`
	SessionQuestionID int                    `db:"session_question_id"`
	QuestionOrder     int                    `db:"question_order"`
	IsReview          bool                   `db:"is_review"`
	UserAnswer        *string                `db:"user_answer"`
	IsCorrect         *bool                  `db:"is_correct"`
	QuestionID        int                    `db:"question_id"`
	ContentID         *int                   `db:"content_id"`
	QuestionType      model.QuestionType     `db:"question_type"`
	Language          string                 `db:"language"`
	ProficiencyLevel  string                 `db:"proficiency_level"`
	Category          model.QuestionCategory `db:"category"`
	Prompt            string                 `db:"prompt"`
	Options           json.RawMessage        `db:"options"`
	CorrectAnswer     string                 `db:"correct_answer"`
	Explanation       string                 `db:"explanation"`
	AudioPath         *string                `db:"audio_path"`
	Difficulty        int                    `db:"difficulty"`
	TimesServed       int                    `db:"times_served"`
	TimesCorrect      int                    `db:"times_correct"`
	EaseFactor        float64                `db:"ease_factor"`
	IntervalDays      int                    `db:"interval_days"`
	Repetitions       int                    `db:"repetitions"`
	NextReviewAt      *time.Time             `db:"next_review_at"`
	LastReviewedAt    *time.Time             `db:"last_reviewed_at"`
	QuestionCreatedAt time.Time              `db:"question_created_at"`
}

// LoadActiveSession loads the full ordered session state in one DB round-trip.
func (r *ActiveSessionRepository) LoadActiveSession(ctx context.Context, sessionID int) (*model.ActiveSessionState, error) {
	var rows []activeSessionRow
	if err := r.db.SelectContext(ctx, &rows, `
		SELECT
			s.id AS session_id,
			s.user_id,
			s.type AS session_type,
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
			q.times_served,
			q.times_correct,
			q.ease_factor,
			q.interval_days,
			q.repetitions,
			q.next_review_at,
			q.last_reviewed_at,
			q.created_at AS question_created_at
		FROM sessions s
		JOIN session_questions sq ON sq.session_id = s.id
		JOIN questions q ON q.id = sq.question_id
		WHERE s.id = $1
		ORDER BY sq.question_order
	`, sessionID); err != nil {
		return nil, fmt.Errorf("ActiveSessionRepository.LoadActiveSession session_id=%d: %w", sessionID, err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("ActiveSessionRepository.LoadActiveSession session_id=%d: %w", sessionID, sql.ErrNoRows)
	}

	first := rows[0]
	state := &model.ActiveSessionState{
		Version: model.ActiveSessionStateVersion,
		Session: model.Session{
			ID:             first.SessionID,
			UserID:         first.UserID,
			Type:           first.SessionType,
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
				TimesServed:      row.TimesServed,
				TimesCorrect:     row.TimesCorrect,
				EaseFactor:       row.EaseFactor,
				IntervalDays:     row.IntervalDays,
				Repetitions:      row.Repetitions,
				NextReviewAt:     row.NextReviewAt,
				LastReviewedAt:   row.LastReviewedAt,
				CreatedAt:        row.QuestionCreatedAt,
			},
		})
	}
	state.RecountAnswered()

	return state, nil
}

// FlushActiveSession persists the active state in a single DB transaction.
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
		if err := flushQuestions(ctx, tx, state.Items); err != nil {
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
		return false, fmt.Errorf("ActiveSessionRepository.markSessionCompleted session_id=%d: %w", state.Session.ID, err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("ActiveSessionRepository.markSessionCompleted rows session_id=%d: %w", state.Session.ID, err)
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

type questionFlushRow struct {
	Question     model.Question
	ServedDelta  int
	CorrectDelta int
}

func flushQuestions(ctx context.Context, tx *sqlx.Tx, items []model.ActiveSessionQuestion) error {
	rowsByID := make(map[int]*questionFlushRow)
	ids := make([]int, 0, len(items))
	for _, item := range items {
		if item.SessionQuestion.IsCorrect == nil {
			continue
		}

		row, ok := rowsByID[item.Question.ID]
		if !ok {
			row = &questionFlushRow{Question: item.Question}
			rowsByID[item.Question.ID] = row
			ids = append(ids, item.Question.ID)
		}
		row.Question = item.Question
		row.ServedDelta++
		if *item.SessionQuestion.IsCorrect {
			row.CorrectDelta++
		}
	}
	if len(ids) == 0 {
		return nil
	}

	var values strings.Builder
	args := make([]any, 0, len(ids)*8)
	for i, id := range ids {
		if i > 0 {
			values.WriteString(",")
		}
		base := i * 8
		values.WriteString(fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)",
			base+1, base+2, base+3, base+4, base+5, base+6, base+7, base+8))

		row := rowsByID[id]
		q := row.Question
		args = append(args,
			q.ID,
			row.ServedDelta,
			row.CorrectDelta,
			q.EaseFactor,
			q.IntervalDays,
			q.Repetitions,
			q.NextReviewAt,
			q.LastReviewedAt,
		)
	}

	query := fmt.Sprintf(`
		UPDATE questions AS q
		SET times_served = q.times_served + v.served_delta::int,
			times_correct = q.times_correct + v.correct_delta::int,
			ease_factor = v.ease_factor::double precision,
			interval_days = v.interval_days::int,
			repetitions = v.repetitions::int,
			next_review_at = v.next_review_at::timestamptz,
			last_reviewed_at = v.last_reviewed_at::timestamptz
		FROM (VALUES %s) AS v(
			id, served_delta, correct_delta, ease_factor, interval_days,
			repetitions, next_review_at, last_reviewed_at
		)
		WHERE q.id = v.id::int
	`, values.String())
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("ActiveSessionRepository.flushQuestions count=%d: %w", len(ids), err)
	}
	return nil
}
