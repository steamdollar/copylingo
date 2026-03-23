package repository

import (
	"context"

	"github.com/jmoiron/sqlx"
	"github.com/lsj/copylingo/internal/model"
)

type SessionRepository struct {
	db *sqlx.DB
}

func NewSessionRepository(db *sqlx.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

func (r *SessionRepository) Create(ctx context.Context, s *model.Session) error {
	return r.db.QueryRowContext(ctx, `
		INSERT INTO sessions (user_id, type, status, total_questions)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, s.UserID, s.Type, s.Status, s.TotalQuestions).Scan(&s.ID)
}

func (r *SessionRepository) GetByID(ctx context.Context, id int) (*model.Session, error) {
	s := &model.Session{}
	err := r.db.GetContext(ctx, s, `SELECT * FROM sessions WHERE id = $1`, id)
	return s, err
}

// GetPendingSessions returns all pending sessions for a user.
func (r *SessionRepository) GetPendingSessions(ctx context.Context, userID int64) ([]model.Session, error) {
	var sessions []model.Session
	err := r.db.SelectContext(ctx, &sessions, `
		SELECT * FROM sessions
		WHERE user_id = $1 AND status = 'pending'
		ORDER BY created_at DESC
	`, userID)
	return sessions, err
}

// Start marks a session as in_progress.
func (r *SessionRepository) Start(ctx context.Context, id int) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE sessions SET status = 'in_progress', started_at = NOW() WHERE id = $1
	`, id)
	return err
}

// Complete marks a session as completed with results.
func (r *SessionRepository) Complete(ctx context.Context, id int, correctCount int) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE sessions SET
			status = 'completed', correct_count = $2, completed_at = NOW()
		WHERE id = $1
	`, id, correctCount)
	return err
}

// GetTodaySessions returns today's sessions for a user.
func (r *SessionRepository) GetTodaySessions(ctx context.Context, userID int64) ([]model.Session, error) {
	var sessions []model.Session
	err := r.db.SelectContext(ctx, &sessions, `
		SELECT * FROM sessions
		WHERE user_id = $1 AND created_at::date = CURRENT_DATE
		ORDER BY created_at
	`, userID)
	return sessions, err
}
