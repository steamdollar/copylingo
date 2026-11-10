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
func (processor *PassThroughProcessor) Process(
	ctx context.Context,
	raw []RawContent,
) ([]model.Content, error) {
	contents := make(
		[]model.Content,
		0,
		len(raw),
	)

	for _, rawContent := range raw {
		contents = append(
			contents,
			model.Content{
				SourceType:       model.ContentSourceType(rawContent.SourceType),
				SourceURL:        rawContent.SourceURL,
				Title:            rawContent.Title,
				Body:             rawContent.Body,
				Language:         rawContent.Language,
				ProficiencyLevel: rawContent.Level,
				Difficulty:       rawContent.Difficulty,
				Tags:             rawContent.Tags,
				IsArticle:        rawContent.IsArticle,
				CollectedAt:      time.Now(),
			},
		)
	}

	return contents, nil
}
