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
func (r *SessionQuestionRepository) CreateSessionQuestions(
	ctx context.Context,
	sqs []model.SessionQuestion,
) error {
	if len(sqs) == 0 {
		return nil
	}

	if _, err := r.db.NamedExecContext(
		ctx,
		`
		INSERT INTO session_questions (session_id, question_id, question_order, is_review)
		VALUES (:session_id, :question_id, :question_order, :is_review)
	`,
		sqs,
	); err != nil {
		return fmt.Errorf(
			"SessionQuestionRepository.CreateSessionQuestions count=%d: %w",
			len(sqs),
			err,
		)
	}

	return nil
}

// GetCategoryAccuracy returns accuracy rate per category for answered questions.
func (r *SessionQuestionRepository) GetCategoryAccuracy(
	ctx context.Context,
	userID int64,
) (map[string]float64, error) {
	type row struct {
		Category string  `db:"category"`
		Accuracy float64 `db:"accuracy"`
	}
	var rows []row
	if err := r.db.SelectContext(
		ctx,
		&rows,
		`
		SELECT q.category,
			CASE WHEN COUNT(*) = 0 THEN 0
			ELSE ROUND(COUNT(*) FILTER (WHERE sq.is_correct) * 100.0 / COUNT(*), 1)
			END as accuracy
		FROM session_questions sq
		JOIN sessions s ON s.id = sq.session_id
		JOIN questions q ON q.id = sq.question_id
		WHERE sq.is_correct IS NOT NULL AND s.user_id = $1
		GROUP BY q.category
	`,
		userID,
	); err != nil {
		return nil, fmt.Errorf(
			"get category accuracy user_id=%d: %w",
			userID,
			err,
		)
	}

	result := make(map[string]float64)
	for _, r := range rows {
		result[r.Category] = r.Accuracy
	}
	return result, nil
}

// GetTodayStats returns today's answer stats.
func (r *SessionQuestionRepository) GetTodayStats(
	ctx context.Context,
	userID int64,
) (int, int, error) {
	type stats struct {
		Total   int `db:"total"`
		Correct int `db:"correct"`
	}
	var s stats
	if err := r.db.GetContext(
		ctx,
		&s,
		`
		SELECT COUNT(*) as total, COUNT(*) FILTER (WHERE sq.is_correct) as correct
		FROM session_questions sq
		JOIN sessions s ON s.id = sq.session_id
		WHERE sq.is_correct IS NOT NULL
		  AND s.user_id = $1
		  AND s.created_at::date = CURRENT_DATE
	`,
		userID,
	); err != nil {
		return 0, 0, fmt.Errorf(
			"get today session question stats user_id=%d: %w",
			userID,
			err,
		)
	}
	return s.Total, s.Correct, nil
}
