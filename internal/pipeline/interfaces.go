package pipeline

import (
	"context"

	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/service"
)

type RawContent struct {
	SourceURL  string
	Title      string
	Body       string
	Language   string // ISO 639-1: 'ja', 'el', 'en'
	Level      string // JLPT: N5-N1, CEFR: A1-C2
	SourceType string // 'news' or 'exam_prep'
	Difficulty int    // 1-10
	Tags       []string
	IsArticle  bool
}

type Fetcher interface {
	Name() string

	// Individual item failures should be logged but not stop the whole fetch.
	Fetch(ctx context.Context) ([]RawContent, error)
}

type Processor interface {
	Process(
		ctx context.Context,
		raw []RawContent,
	) ([]model.Content, error)
}

type SaveResult = service.ContentSaveResult

type Saver interface {
	// Duplicate URLs are skipped without error; per-item save failures don't stop the batch.
	Save(
		ctx context.Context,
		contents []model.Content,
	) (SaveResult, error)
}

type PipelineResult struct {
	FetcherName  string
	FetchedCount int
	SaveResult   SaveResult
	Err          error
}
