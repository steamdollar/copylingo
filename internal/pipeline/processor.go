package pipeline

import (
	"context"
	"time"

	"github.com/lsj/copylingo/internal/model"
)

type PassThroughProcessor struct{}

func NewPassThroughProcessor() *PassThroughProcessor {
	return &PassThroughProcessor{}
}

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
