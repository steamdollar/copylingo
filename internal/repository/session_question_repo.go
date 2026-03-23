package repository

import (
	"context"

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

// Create inserts a session question entry.
func (r *SessionQuestionRepository) Create(ctx context.Context, sq *model.SessionQuestion) error {
	return r.db.QueryRowContext(ctx, `
		INSERT INTO session_questions (session_id, question_id, question_order, is_review)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, sq.SessionID, sq.QuestionID, sq.QuestionOrder, sq.IsReview).Scan(&sq.ID)
}

// CreateBatch inserts multiple session question entries.
func (r *SessionQuestionRepository) CreateBatch(ctx context.Context, sqs []model.SessionQuestion) error {
	for i := range sqs {
		if err := r.Create(ctx, &sqs[i]); err != nil {
			return err
		}
	}
	return nil
}

// GetBySession returns all questions for a session, ordered.
func (r *SessionQuestionRepository) GetBySession(ctx context.Context, sessionID int) ([]model.SessionQuestion, error) {
	var sqs []model.SessionQuestion
	err := r.db.SelectContext(ctx, &sqs, `
		SELECT * FROM session_questions WHERE session_id = $1 ORDER BY question_order
	`, sessionID)
	return sqs, err
}

// RecordAnswer updates the user's answer for a session question.
func (r *SessionQuestionRepository) RecordAnswer(ctx context.Context, sessionID, questionID int, userAnswer string, isCorrect bool) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE session_questions
		SET user_answer = $3, is_correct = $4
		WHERE session_id = $1 AND question_id = $2
	`, sessionID, questionID, userAnswer, isCorrect)
	return err
}

// GetWrongAnswers returns wrong answers for a session.
func (r *SessionQuestionRepository) GetWrongAnswers(ctx context.Context, sessionID int) ([]model.SessionQuestion, error) {
	var sqs []model.SessionQuestion
	err := r.db.SelectContext(ctx, &sqs, `
		SELECT * FROM session_questions
		WHERE session_id = $1 AND is_correct = FALSE
		ORDER BY question_order
	`, sessionID)
	return sqs, err
}

// GetCategoryAccuracy returns accuracy rate per category for answered questions.
func (r *SessionQuestionRepository) GetCategoryAccuracy(ctx context.Context) (map[string]float64, error) {
	type row struct {
		Category string  `db:"category"`
		Accuracy float64 `db:"accuracy"`
	}
	var rows []row
	err := r.db.SelectContext(ctx, &rows, `
		SELECT q.category,
			CASE WHEN COUNT(*) = 0 THEN 0
			ELSE ROUND(COUNT(*) FILTER (WHERE sq.is_correct) * 100.0 / COUNT(*), 1)
			END as accuracy
		FROM session_questions sq
		JOIN questions q ON q.id = sq.question_id
		WHERE sq.is_correct IS NOT NULL
		GROUP BY q.category
	`)
	if err != nil {
		return nil, err
	}

	result := make(map[string]float64)
	for _, r := range rows {
		result[r.Category] = r.Accuracy
	}
	return result, nil
}

// GetTodayStats returns today's answer stats.
func (r *SessionQuestionRepository) GetTodayStats(ctx context.Context) (total int, correct int, err error) {
	type stats struct {
		Total   int `db:"total"`
		Correct int `db:"correct"`
	}
	var s stats
	err = r.db.GetContext(ctx, &s, `
		SELECT COUNT(*) as total, COUNT(*) FILTER (WHERE sq.is_correct) as correct
		FROM session_questions sq
		JOIN sessions s ON s.id = sq.session_id
		WHERE sq.is_correct IS NOT NULL AND s.created_at::date = CURRENT_DATE
	`)
	return s.Total, s.Correct, err
}
