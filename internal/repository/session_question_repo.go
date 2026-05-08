package repository

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/lsj/copylingo/internal/model"
)

// SessionQuestionRepository manages the session_questions join table.
type SessionQuestionRepository struct {
	db *sqlx.DB
}

func NewSessionQuestionRepository(db *sqlx.DB) *SessionQuestionRepository {
	return &SessionQuestionRepository{db: db}
}

// CreateSessionQuestions inserts multiple session question entries.
func (r *SessionQuestionRepository) CreateSessionQuestions(ctx context.Context, sqs []model.SessionQuestion) error {
	if len(sqs) == 0 {
		return nil
	}

	if _, err := r.db.NamedExecContext(ctx, `
		INSERT INTO session_questions (session_id, question_id, question_order, is_review)
		VALUES (:session_id, :question_id, :question_order, :is_review)
	`, sqs); err != nil {
		return fmt.Errorf("SessionQuestionRepository.CreateSessionQuestions count=%d: %w", len(sqs), err)
	}

	return nil
}

// GetBySession returns all questions for a session, ordered.
func (r *SessionQuestionRepository) GetBySession(ctx context.Context, sessionID int) ([]model.SessionQuestion, error) {
	var sqs []model.SessionQuestion
	if err := r.db.SelectContext(ctx, &sqs, `
		SELECT * FROM session_questions WHERE session_id = $1 ORDER BY question_order
	`, sessionID); err != nil {
		return nil, fmt.Errorf("get session questions by session session_id=%d: %w", sessionID, err)
	}
	return sqs, nil
}

// RecordAnswer updates the user's answer for a session question.
func (r *SessionQuestionRepository) RecordAnswer(ctx context.Context, sessionID, questionID int, userAnswer string, isCorrect bool) error {
	if _, err := r.db.ExecContext(ctx, `
		UPDATE session_questions
		SET user_answer = $3, is_correct = $4
		WHERE session_id = $1 AND question_id = $2
	`, sessionID, questionID, userAnswer, isCorrect); err != nil {
		return fmt.Errorf("record session question answer session_id=%d question_id=%d: %w",
			sessionID, questionID, err)
	}
	return nil
}

// GetWrongAnswers returns wrong answers for a session.
func (r *SessionQuestionRepository) GetWrongAnswers(ctx context.Context, sessionID int) ([]model.SessionQuestion, error) {
	var sqs []model.SessionQuestion
	if err := r.db.SelectContext(ctx, &sqs, `
		SELECT * FROM session_questions
		WHERE session_id = $1 AND is_correct = FALSE
		ORDER BY question_order
	`, sessionID); err != nil {
		return nil, fmt.Errorf("get wrong session question answers session_id=%d: %w", sessionID, err)
	}
	return sqs, nil
}

// GetCategoryAccuracy returns accuracy rate per category for answered questions.
func (r *SessionQuestionRepository) GetCategoryAccuracy(ctx context.Context) (map[string]float64, error) {
	type row struct {
		Category string  `db:"category"`
		Accuracy float64 `db:"accuracy"`
	}
	var rows []row
	if err := r.db.SelectContext(ctx, &rows, `
		SELECT q.category,
			CASE WHEN COUNT(*) = 0 THEN 0
			ELSE ROUND(COUNT(*) FILTER (WHERE sq.is_correct) * 100.0 / COUNT(*), 1)
			END as accuracy
		FROM session_questions sq
		JOIN questions q ON q.id = sq.question_id
		WHERE sq.is_correct IS NOT NULL
		GROUP BY q.category
	`); err != nil {
		return nil, fmt.Errorf("get category accuracy: %w", err)
	}

	result := make(map[string]float64)
	for _, r := range rows {
		result[r.Category] = r.Accuracy
	}
	return result, nil
}

// GetTodayStats returns today's answer stats.
func (r *SessionQuestionRepository) GetTodayStats(ctx context.Context) (int, int, error) {
	type stats struct {
		Total   int `db:"total"`
		Correct int `db:"correct"`
	}
	var s stats
	if err := r.db.GetContext(ctx, &s, `
		SELECT COUNT(*) as total, COUNT(*) FILTER (WHERE sq.is_correct) as correct
		FROM session_questions sq
		JOIN sessions s ON s.id = sq.session_id
		WHERE sq.is_correct IS NOT NULL AND s.created_at::date = CURRENT_DATE
	`); err != nil {
		return 0, 0, fmt.Errorf("get today session question stats: %w", err)
	}
	return s.Total, s.Correct, nil
}
