package repository

import (
	"context"

	"github.com/jmoiron/sqlx"

	"github.com/lsj/copylingo/internal/model"
)

type ContentRepository struct {
	db *sqlx.DB
}

func NewContentRepository(db *sqlx.DB) *ContentRepository {
	return &ContentRepository{db: db}
}

func (r *ContentRepository) Create(
	ctx context.Context,
	content *model.Content,
) error {
	return r.db.QueryRowContext(
		ctx,
		`
		INSERT INTO contents (source_type, source_url, title, body, language, proficiency_level, difficulty, tags, is_article, collected_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`,
		content.SourceType,
		content.SourceURL,
		content.Title,
		content.Body,
		content.Language,
		content.ProficiencyLevel,
		content.Difficulty,
		content.Tags,
		content.IsArticle,
		content.CollectedAt,
	).Scan(&content.ID)
}

func (r *ContentRepository) GetByID(
	ctx context.Context,
	id int,
) (*model.Content, error) {
	content := &model.Content{}
	err := r.db.GetContext(
		ctx,
		content,
		`SELECT * FROM contents WHERE id = $1`,
		id,
	)
	return content, err
}

// ExistsByURL checks if content with the given URL already exists.
func (r *ContentRepository) ExistsByURL(
	ctx context.Context,
	url string,
) (bool, error) {
	var count int
	err := r.db.GetContext(
		ctx,
		&count,
		`SELECT COUNT(*) FROM contents WHERE source_url = $1`,
		url,
	)
	return count > 0, err
}
