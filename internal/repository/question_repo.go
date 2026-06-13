package repository

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"

	"github.com/lsj/copylingo/internal/model"
)

type QuestionRepository struct {
	db *sqlx.DB
}

func NewQuestionRepository(db *sqlx.DB) *QuestionRepository {
	return &QuestionRepository{db: db}
}

// CreateBatch inserts multiple questions in a single transaction and round-trip.
func (r *QuestionRepository) CreateBatch(ctx context.Context, questions []*model.Question) error {
	if len(questions) == 0 {
		return nil
	}

	query, args := buildQuestionBatchInsertQuery(questions)
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		log.Println("QuestionBatch insert failed:", err)
		return err
	}

	return nil
}

func (r *QuestionRepository) GetByID(ctx context.Context, id int) (*model.Question, error) {
	q := &model.Question{}
	err := r.db.GetContext(ctx, q, `SELECT * FROM questions WHERE id = $1`, id)
	return q, err
}

// GetNewQuestions returns questions that haven't been reviewed yet (next_review_at IS NULL).
func (r *QuestionRepository) GetNewQuestions(
	ctx context.Context,
	language, level, category string,
	excludeIDs []int,
	limit int,
) ([]model.Question, error) {
	var questions []model.Question
	err := r.db.SelectContext(ctx, &questions, `
		SELECT * FROM questions
		WHERE language = $1 AND proficiency_level = $2
		AND ($3 = '' OR category = $3)
		AND NOT (id = ANY(COALESCE($4::int[], '{}')))
		AND next_review_at IS NULL
		ORDER BY difficulty ASC, RANDOM()
		LIMIT $5
	`, language, level, category, pq.Array(excludeIDs), limit)
	return questions, err
}

// GetDueReviews returns questions due for SRS review.
func (r *QuestionRepository) GetDueReviews(ctx context.Context, limit int) ([]model.Question, error) {
	var questions []model.Question
	err := r.db.SelectContext(ctx, &questions, `
		SELECT * FROM questions
		WHERE next_review_at IS NOT NULL AND next_review_at <= NOW()
		ORDER BY next_review_at ASC
		LIMIT $1
	`, limit)
	return questions, err
}

// GetDueReviewCount returns the number of questions due for review.
func (r *QuestionRepository) GetDueReviewCount(ctx context.Context) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `
		SELECT COUNT(*) FROM questions
		WHERE next_review_at IS NOT NULL AND next_review_at <= NOW()
	`)
	return count, err
}

// UpdateSRS updates the SRS state of a question.
func (r *QuestionRepository) UpdateSRS(ctx context.Context, q *model.Question) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE questions SET
			ease_factor = $2, interval_days = $3, repetitions = $4,
			next_review_at = $5, last_reviewed_at = $6
		WHERE id = $1
	`, q.ID, q.EaseFactor, q.IntervalDays, q.Repetitions, q.NextReviewAt, q.LastReviewedAt)
	return err
}

// IncrementServed increments the times_served counter.
func (r *QuestionRepository) IncrementServed(ctx context.Context, id int) error {
	_, err := r.db.ExecContext(ctx, `UPDATE questions SET times_served = times_served + 1 WHERE id = $1`, id)
	return err
}

// IncrementCorrect increments the times_correct counter.
func (r *QuestionRepository) IncrementCorrect(ctx context.Context, id int) error {
	_, err := r.db.ExecContext(ctx, `UPDATE questions SET times_correct = times_correct + 1 WHERE id = $1`, id)
	return err
}

func buildQuestionBatchInsertQuery(questions []*model.Question) (string, []any) {
	const columnCount = 12

	var query strings.Builder
	query.WriteString(`
		INSERT INTO questions (content_id, type, item_type, language, proficiency_level, category, prompt, options, correct_answer, explanation, audio_path, difficulty)
		VALUES
	`)

	args := make([]any, 0, len(questions)*columnCount)
	for i, q := range questions {
		if i > 0 {
			query.WriteString(",")
		}

		base := i * columnCount
		query.WriteString(fmt.Sprintf(
			"($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
			base+1, base+2, base+3, base+4, base+5, base+6,
			base+7, base+8, base+9, base+10, base+11, base+12,
		))

		args = append(args,
			q.ContentID,
			q.Type,
			q.Skill,
			q.Language,
			q.ProficiencyLevel,
			q.Category,
			q.Prompt,
			q.Options,
			q.CorrectAnswer,
			q.Explanation,
			q.AudioPath,
			q.Difficulty,
		)
	}

	return query.String(), args
}
