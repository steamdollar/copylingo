package repository

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
)

type SessionRepository struct {
	db *sqlx.DB
}

func NewSessionRepository(db *sqlx.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

func (r *SessionRepository) CreateSession(ctx context.Context, s *model.Session) error {
	if !s.Mode.IsValid() {
		return fmt.Errorf("SessionRepository.CreateSession user_id=%d type=%s invalid mode=%q",
			s.UserID, s.Type, s.Mode)
	}
	if err := r.db.QueryRowContext(ctx, `
		INSERT INTO sessions (user_id, type, mode, status, total_questions)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, s.UserID, s.Type, s.Mode, s.Status, s.TotalQuestions).Scan(&s.ID); err != nil {
		return fmt.Errorf("SessionRepository.CreateSession user_id=%d type=%s mode=%s: %w",
			s.UserID, s.Type, s.Mode, err)
	}
	return nil
}

func (r *SessionRepository) GetByID(ctx context.Context, id int) (*model.Session, error) {
	s := &model.Session{}
	if err := r.db.GetContext(ctx, s, `SELECT * FROM sessions WHERE id = $1`, id); err != nil {
		return nil, fmt.Errorf("SessionRepository.GetByID id=%d: %w", id, err)
	}
	return s, nil
}

func (r *SessionRepository) GetSessionsByStatus(
	ctx context.Context,
	userID int64,
	status config.SessionStatus,
) ([]model.Session, error) {
	var sessions []model.Session
	if err := r.db.SelectContext(ctx, &sessions, `
		SELECT * FROM sessions
		WHERE user_id = $1 AND status = $2
		ORDER BY started_at DESC NULLS LAST, created_at DESC
	`, userID, status); err != nil {
		return nil, fmt.Errorf("SessionRepository.GetSessionsByStatus user_id=%d status=%s: %w",
			userID, status, err)
	}
	return sessions, nil
}

// ListInProgress returns all in-progress sessions for all users.
func (r *SessionRepository) ListInProgress(ctx context.Context) ([]model.Session, error) {
	var sessions []model.Session
	if err := r.db.SelectContext(ctx, &sessions, `
		SELECT * FROM sessions
		WHERE status = 'in_progress' AND mode = 'quiz'
		ORDER BY started_at DESC NULLS LAST, created_at DESC
	`); err != nil {
		return nil, fmt.Errorf("SessionRepository.ListInProgress: %w", err)
	}
	return sessions, nil
}

// Start marks a session as in_progress.
func (r *SessionRepository) Start(ctx context.Context, id int) error {
	if _, err := r.db.ExecContext(ctx, `
		UPDATE sessions SET status = 'in_progress', started_at = NOW() WHERE id = $1
	`, id); err != nil {
		return fmt.Errorf("SessionRepository.Start id=%d: %w", id, err)
	}
	return nil
}

func (r *SessionRepository) Complete(ctx context.Context, id int, correctCount int) error {
	if _, err := r.db.ExecContext(ctx, `
		UPDATE sessions SET
			status = 'completed', correct_count = $2, completed_at = NOW()
		WHERE id = $1
	`, id, correctCount); err != nil {
		return fmt.Errorf("SessionRepository.Complete id=%d: %w", id, err)
	}
	return nil
}

func (r *SessionRepository) GetTodaySessions(ctx context.Context, userID int64) ([]model.Session, error) {
	var sessions []model.Session
	if err := r.db.SelectContext(ctx, &sessions, `
		SELECT * FROM sessions
		WHERE user_id = $1 AND created_at::date = CURRENT_DATE
		ORDER BY created_at
	`, userID); err != nil {
		return nil, fmt.Errorf("SessionRepository.GetTodaySessions user_id=%d: %w", userID, err)
	}
	return sessions, nil
}
