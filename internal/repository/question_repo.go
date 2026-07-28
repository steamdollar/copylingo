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

const questionCatalogColumns = `
	q.id, q.question_key, q.content_id, q.material_id, q.type, q.item_type,
	q.language, q.proficiency_level, q.category, q.prompt, q.options,
	q.correct_answer, q.explanation, q.audio_path, q.audio_script,
	q.audio_file_id, q.difficulty, q.created_at`

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
	err := r.db.GetContext(
		ctx,
		q,
		fmt.Sprintf(`SELECT %s FROM questions q WHERE q.id = $1`, questionCatalogColumns),
		id,
	)
	return q, err
}

// GetNewQuestions returns catalog questions without progress for the user.
func (r *QuestionRepository) GetNewQuestions(
	ctx context.Context,
	userID int64,
	language, level, category string,
	excludeIDs []int,
	limit, kanjiRecallLimit int,
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
		kanjiRecallLimit,
	)
	return questions, err
}

var newQuestionsForStudiedMaterialsQuery = fmt.Sprintf(`
		WITH candidates AS (
			SELECT
				q.id,
				CASE WHEN ump.material_id IS NOT NULL THEN 0 ELSE 1 END AS material_priority,
				q.difficulty,
				RANDOM() AS random_order,
				CASE WHEN q.item_type = 'vocab_kanji_recall' THEN
					ROW_NUMBER() OVER (
						PARTITION BY q.item_type
						ORDER BY
							CASE WHEN ump.material_id IS NOT NULL THEN 0 ELSE 1 END,
							q.difficulty ASC,
							RANDOM()
					)
				ELSE 1 END AS kanji_recall_rank
			FROM questions q
			LEFT JOIN user_material_progress ump
				ON ump.material_id = q.material_id
				AND ump.user_id = $1
				AND ump.times_studied > 0
			LEFT JOIN user_question_progress uqp
				ON uqp.question_id = q.id
				AND uqp.user_id = $1
			WHERE q.language = $2 AND q.proficiency_level = $3
			AND ($4 = '' OR q.category = $4)
			AND NOT (q.id = ANY(COALESCE($5::int[], '{}')))
			AND uqp.question_id IS NULL
			-- Listening questions are only servable once their audio has been generated.
			AND (q.category <> 'listening' OR q.audio_path IS NOT NULL)
			-- Reading questions become new candidates only after the user studied
			-- their passage material; other categories may fall back to unstudied
			-- materials (ADR-036).
			AND (q.category <> 'reading' OR ump.material_id IS NOT NULL)
		)
		SELECT %s
		FROM questions q
		JOIN candidates candidate ON candidate.id = q.id
		WHERE q.item_type IS DISTINCT FROM 'vocab_kanji_recall'
			OR candidate.kanji_recall_rank <= $7
		ORDER BY
			candidate.material_priority,
			candidate.difficulty ASC,
			candidate.random_order
		LIMIT $6
	`, questionCatalogColumns)

// GetListeningNeedingAudio returns listening questions that have a script but no
// generated audio yet, oldest first, so the pre-generation pipeline can fill them
// in bounded batches (ADR-031 push model). audio_file_id is ignored here — the
// gate is audio_path (the object-store SSOT pointer).
func (r *QuestionRepository) GetListeningNeedingAudio(
	ctx context.Context,
	language, level string,
	limit int,
) ([]model.Question, error) {
	var questions []model.Question
	err := r.db.SelectContext(ctx, &questions, fmt.Sprintf(`
		SELECT %s FROM questions q
		WHERE category = 'listening'
		  AND language = $1 AND proficiency_level = $2
		  AND audio_script IS NOT NULL AND audio_script <> ''
		  AND audio_path IS NULL
		ORDER BY created_at ASC
		LIMIT $3
	`, questionCatalogColumns), language, level, limit)
	return questions, err
}

// SetAudioPath stores the object-store key produced for a question's audio.
func (r *QuestionRepository) SetAudioPath(ctx context.Context, id int, audioPath string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE questions SET audio_path = $2 WHERE id = $1`, id, audioPath)
	return err
}

// SetAudioFileID caches the Telegram file_id returned after the first upload so
// later sends can reuse it (ADR-032). The object store remains the SSOT.
func (r *QuestionRepository) SetAudioFileID(ctx context.Context, id int, fileID string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE questions SET audio_file_id = $2 WHERE id = $1`, id, fileID)
	return err
}

// GetDueReviews returns questions due for SRS review.
func (r *QuestionRepository) GetDueReviews(
	ctx context.Context,
	userID int64,
	language, level string,
	limit, kanjiRecallLimit int,
) ([]model.Question, error) {
	var questions []model.Question
	err := r.db.SelectContext(
		ctx,
		&questions,
		dueReviewsForStudiedMaterialsQuery,
		userID,
		language,
		level,
		limit,
		kanjiRecallLimit,
	)
	return questions, err
}

var dueReviewsForStudiedMaterialsQuery = fmt.Sprintf(`
		WITH candidates AS (
			SELECT
				q.id,
				CASE WHEN ump.material_id IS NOT NULL THEN 0 ELSE 1 END AS material_priority,
				uqp.next_review_at,
				CASE WHEN q.item_type = 'vocab_kanji_recall' THEN
					ROW_NUMBER() OVER (
						PARTITION BY q.item_type
						ORDER BY
							CASE WHEN ump.material_id IS NOT NULL THEN 0 ELSE 1 END,
							uqp.next_review_at ASC
					)
				ELSE 1 END AS kanji_recall_rank
			FROM user_question_progress uqp
			JOIN questions q ON q.id = uqp.question_id
			LEFT JOIN user_material_progress ump
				ON ump.material_id = q.material_id
				AND ump.user_id = $1
				AND ump.times_studied > 0
			WHERE uqp.user_id = $1
			  AND uqp.next_review_at IS NOT NULL
			  AND uqp.next_review_at <= NOW()
			  AND q.language = $2
			  AND q.proficiency_level = $3
		)
		SELECT %s
		FROM questions q
		JOIN candidates candidate ON candidate.id = q.id
		WHERE q.item_type IS DISTINCT FROM 'vocab_kanji_recall'
			OR candidate.kanji_recall_rank <= $5
		ORDER BY
			candidate.material_priority,
			candidate.next_review_at ASC
		LIMIT $4
	`, questionCatalogColumns)

// GetDueReviewCount returns the number of questions due for review.
func (r *QuestionRepository) GetDueReviewCount(
	ctx context.Context,
	userID int64,
	language, level string,
) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `
		SELECT COUNT(*)
		FROM user_question_progress uqp
		JOIN questions q ON q.id = uqp.question_id
		WHERE uqp.user_id = $1
		  AND uqp.next_review_at IS NOT NULL
		  AND uqp.next_review_at <= NOW()
		  AND q.language = $2
		  AND q.proficiency_level = $3
	`, userID, language, level)
	return count, err
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
			-- A seed normally has no generated-audio pointer. Keep the existing
			-- runtime value when its script is unchanged; otherwise force a new
			-- generation for the changed script.
			audio_path = CASE
				WHEN questions.audio_script IS NOT DISTINCT FROM EXCLUDED.audio_script
					THEN COALESCE(EXCLUDED.audio_path, questions.audio_path)
				ELSE EXCLUDED.audio_path
			END,
			audio_script = EXCLUDED.audio_script,
			difficulty = EXCLUDED.difficulty
	`
	return query, args
}

func buildQuestionBatchBaseQuery(questions []*model.Question) (string, []any) {
	const columnCount = 15

	var query strings.Builder
	query.WriteString(`
		INSERT INTO questions (
			question_key, content_id, material_id, type, item_type, language, proficiency_level,
			category, prompt, options, correct_answer, explanation, audio_path, audio_script, difficulty
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
			"($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
			base+1, base+2, base+3, base+4, base+5, base+6, base+7, base+8,
			base+9, base+10, base+11, base+12, base+13, base+14, base+15,
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
			q.AudioScript,
			q.Difficulty,
		)
	}

	return query.String(), args
}
