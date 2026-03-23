package pipeline

import (
	"context"

	"github.com/lsj/copylingo/internal/model"
)

// RawContent represents raw data fetched from external sources.
// This intermediate structure allows Fetchers to remain decoupled from model.Content.
type RawContent struct {
	SourceURL  string
	Title      string
	Body       string
	Language   string   // ISO 639-1: 'ja', 'el', 'en'
	Level      string   // JLPT: N5-N1, CEFR: A1-C2
	SourceType string   // 'news' or 'exam_prep'
	Difficulty int      // 1-10
	Tags       []string // e.g., ["news", "nhk"]
	IsArticle  bool     // true for reading practice content
}

// Fetcher collects raw content from external sources.
type Fetcher interface {
	// Name returns a unique identifier for this fetcher.
	Name() string

	// Fetch retrieves content from the external source.
	// Individual item failures should be logged but not stop the entire fetch.
	Fetch(ctx context.Context) ([]RawContent, error)
}

// Processor transforms raw content into model.Content.
// This allows for AI processing, enrichment, or pass-through.
type Processor interface {
	// Process transforms raw content into model.Content.
	// For Phase 2.1, this is a simple pass-through.
	// For Phase 2.3, this will involve AI processing to generate questions.
	Process(ctx context.Context, raw []RawContent) ([]model.Content, error)
}

// SaveResult contains the outcome of a save operation.
type SaveResult struct {
	Saved      int     // Number of items successfully saved
	Duplicates int     // Number of items skipped due to duplicate URL
	Errors     []error // Individual save errors (non-fatal)
}

// Saver persists processed content to the database.
type Saver interface {
	// Save persists content to the database.
	// Duplicate URLs are skipped (not treated as errors).
	// Individual save failures are collected but don't stop the batch.
	Save(ctx context.Context, contents []model.Content) (SaveResult, error)
}

// PipelineResult contains the outcome of a single pipeline run.
type PipelineResult struct {
	FetcherName  string
	FetchedCount int
	SaveResult   SaveResult
	Err          error // Fatal error that stopped the pipeline
}

// ContentRepository defines the interface for content persistence.
// This allows for mocking in tests.
type ContentRepository interface {
	Create(ctx context.Context, content *model.Content) error
	ExistsByURL(ctx context.Context, url string) (bool, error)
}
