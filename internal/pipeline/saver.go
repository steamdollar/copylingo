package pipeline

import (
	"context"
	"fmt"
	"log"

	"github.com/lsj/copylingo/internal/model"
)

// ContentSaver saves content to the database with duplicate detection.
type ContentSaver struct {
	repo ContentRepository
}

// NewContentSaver creates a new content saver.
func NewContentSaver(repo ContentRepository) *ContentSaver {
	return &ContentSaver{repo: repo}
}

// Save persists content to the database.
// Duplicate URLs are skipped. Individual errors don't stop the batch.
func (s *ContentSaver) Save(ctx context.Context, contents []model.Content) (SaveResult, error) {
	result := SaveResult{
		Errors: make([]error, 0),
	}

	for i := range contents {
		content := &contents[i]

		// Check for duplicate
		exists, err := s.repo.ExistsByURL(ctx, content.SourceURL)
		if err != nil {
			wrappedErr := fmt.Errorf("check exists %s: %w", content.SourceURL, err)
			result.Errors = append(result.Errors, wrappedErr)
			log.Printf("[ContentSaver] WARN: %v", wrappedErr)
			continue
		}

		if exists {
			result.Duplicates++
			continue
		}

		// Save
		if err := s.repo.Create(ctx, content); err != nil {
			wrappedErr := fmt.Errorf("save %s: %w", content.SourceURL, err)
			result.Errors = append(result.Errors, wrappedErr)
			log.Printf("[ContentSaver] WARN: %v", wrappedErr)
			continue
		}

		result.Saved++
	}

	log.Printf("[ContentSaver] INFO: saved=%d, duplicates=%d, errors=%d",
		result.Saved, result.Duplicates, len(result.Errors))

	return result, nil
}
