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

// UpsertSeedBatch inserts or refreshes seed-owned questions identified by stable question_key.
func (r *QuestionRepository) UpsertSeedBatch(ctx context.Context, questions []*model.Question) error {
	if len(questions) == 0 {
		return nil
	}
	for idx, question := range questions {
		if question.QuestionKey == nil || *question.QuestionKey == "" {
			return fmt.Errorf("QuestionRepository.UpsertSeedBatch question_index=%d: missing question_key", idx)
		}
	}

	query, args := buildQuestionBatchUpsertQuery(questions)
	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("QuestionRepository.UpsertSeedBatch count=%d: %w", len(questions), err)
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
	userID int64,
	language, level, category string,
	excludeIDs []int,
	limit int,
) ([]model.Question, error) {
	var questions []model.Question
	err := r.db.SelectContext(
		ctx,
		&questions,
		newQuestionsForStudiedMaterialsQuery,
		userID,
		language,
		level,
		category,
		pq.Array(excludeIDs),
		limit,
	)
	return questions, err
}

const newQuestionsForStudiedMaterialsQuery = `
		SELECT q.*
		FROM questions q
		LEFT JOIN user_material_progress ump
			ON ump.material_id = q.material_id
			AND ump.user_id = $1
			AND ump.times_studied > 0
		WHERE q.language = $2 AND q.proficiency_level = $3
		AND ($4 = '' OR q.category = $4)
		AND NOT (q.id = ANY(COALESCE($5::int[], '{}')))
		AND q.next_review_at IS NULL
		ORDER BY
			CASE WHEN ump.material_id IS NOT NULL THEN 0 ELSE 1 END,
			q.difficulty ASC,
			RANDOM()
		LIMIT $6
	`

// GetDueReviews returns questions due for SRS review.
func (r *QuestionRepository) GetDueReviews(ctx context.Context, userID int64, limit int) ([]model.Question, error) {
	var questions []model.Question
	err := r.db.SelectContext(ctx, &questions, dueReviewsForStudiedMaterialsQuery, userID, limit)
	return questions, err
}

const dueReviewsForStudiedMaterialsQuery = `
		SELECT q.*
		FROM questions q
		LEFT JOIN user_material_progress ump
			ON ump.material_id = q.material_id
			AND ump.user_id = $1
			AND ump.times_studied > 0
		WHERE q.next_review_at IS NOT NULL AND q.next_review_at <= NOW()
		ORDER BY
			CASE WHEN ump.material_id IS NOT NULL THEN 0 ELSE 1 END,
			q.next_review_at ASC
		LIMIT $2
	`

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
	query, args := buildQuestionBatchBaseQuery(questions)
	return query, args
}

func buildQuestionBatchUpsertQuery(questions []*model.Question) (string, []any) {
	query, args := buildQuestionBatchBaseQuery(questions)
	query += `
		ON CONFLICT (question_key) DO UPDATE SET
			content_id = EXCLUDED.content_id,
			material_id = EXCLUDED.material_id,
			type = EXCLUDED.type,
			item_type = EXCLUDED.item_type,
			language = EXCLUDED.language,
			proficiency_level = EXCLUDED.proficiency_level,
			category = EXCLUDED.category,
			prompt = EXCLUDED.prompt,
			options = EXCLUDED.options,
			correct_answer = EXCLUDED.correct_answer,
			explanation = EXCLUDED.explanation,
			audio_path = EXCLUDED.audio_path,
			difficulty = EXCLUDED.difficulty
	`
	return query, args
}

func buildQuestionBatchBaseQuery(questions []*model.Question) (string, []any) {
	const columnCount = 14

	var query strings.Builder
	query.WriteString(`
		INSERT INTO questions (
			question_key, content_id, material_id, type, item_type, language, proficiency_level,
			category, prompt, options, correct_answer, explanation, audio_path, difficulty
		)
		VALUES
	`)

	args := make([]any, 0, len(questions)*columnCount)
	for i, q := range questions {
		if i > 0 {
			query.WriteString(",")
		}

		base := i * columnCount
		query.WriteString(fmt.Sprintf(
			"($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
			base+1, base+2, base+3, base+4, base+5, base+6, base+7,
			base+8, base+9, base+10, base+11, base+12, base+13, base+14,
		))

		args = append(args,
			q.QuestionKey,
			q.ContentID,
			q.MaterialID,
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
