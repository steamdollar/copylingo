package service

import (
	"context"
	"fmt"
	"log"

	"github.com/lsj/copylingo/internal/model"
)

// ContentSaveResult summarizes a batch while keeping individual failures visible.
type ContentSaveResult struct {
	Saved      int
	Duplicates int
	Errors     []error
}

type contentRepository interface {
	Create(ctx context.Context, content *model.Content) error
	ExistsByURL(ctx context.Context, url string) (bool, error)
}

// ContentService owns duplicate detection and persistence for collected content.
type ContentService struct {
	repo contentRepository
}

func NewContentService(repo contentRepository) *ContentService {
	return &ContentService{repo: repo}
}

// Save skips duplicate URLs and records per-item failures without stopping the batch.
func (s *ContentService) Save(ctx context.Context, contents []model.Content) (ContentSaveResult, error) {
	result := ContentSaveResult{Errors: make([]error, 0)}
	for i := range contents {
		content := &contents[i]

		exists, err := s.repo.ExistsByURL(ctx, content.SourceURL)
		if err != nil {
			wrappedErr := fmt.Errorf("check exists %s: %w", content.SourceURL, err)
			result.Errors = append(result.Errors, wrappedErr)
			log.Printf("[ContentService] WARN: %v", wrappedErr)
			continue
		}
		if exists {
			result.Duplicates++
			continue
		}

		if err := s.repo.Create(ctx, content); err != nil {
			wrappedErr := fmt.Errorf("save %s: %w", content.SourceURL, err)
			result.Errors = append(result.Errors, wrappedErr)
			log.Printf("[ContentService] WARN: %v", wrappedErr)
			continue
		}
		result.Saved++
	}

	log.Printf("[ContentService] INFO: saved=%d, duplicates=%d, errors=%d",
		result.Saved, result.Duplicates, len(result.Errors))
	return result, nil
}
