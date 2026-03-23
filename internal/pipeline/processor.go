package pipeline

import (
	"context"
	"time"

	"github.com/lsj/copylingo/internal/model"
)

// PassThroughProcessor converts RawContent to model.Content without modification.
// In Phase 2.3, AIProcessor will be added for question generation.
type PassThroughProcessor struct{}

// NewPassThroughProcessor creates a new pass-through processor.
func NewPassThroughProcessor() *PassThroughProcessor {
	return &PassThroughProcessor{}
}

// Process converts raw content to model.Content.
func (p *PassThroughProcessor) Process(ctx context.Context, raw []RawContent) ([]model.Content, error) {
	contents := make([]model.Content, 0, len(raw))

	for _, r := range raw {
		contents = append(contents, model.Content{
			SourceType:       model.ContentSourceType(r.SourceType),
			SourceURL:        r.SourceURL,
			Title:            r.Title,
			Body:             r.Body,
			Language:         r.Language,
			ProficiencyLevel: r.Level,
			Difficulty:       r.Difficulty,
			Tags:             r.Tags,
			IsArticle:        r.IsArticle,
			CollectedAt:      time.Now(),
		})
	}

	return contents, nil
}
