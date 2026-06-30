package service

import (
	"context"
	"log/slog"
	"math/rand"

	"github.com/lsj/copylingo/internal/external"
	"github.com/lsj/copylingo/internal/model"
)

const (
	// TipBucketTarget is the per-(language, level) active-tip balance the
	// generator fills toward. Once reached, top-up becomes a no-op.
	TipBucketTarget = 50
	// TipGeneratePerCycle is how many tips one TopUpBucket call requests from the LLM.
	TipGeneratePerCycle = 3
	// TipPromptVersion tags generated tips with the prompt revision; bump when
	// the GenerateTips prompt body changes so old and new tips stay distinguishable.
	TipPromptVersion = "v1"
)

// tipGeneratorRepo is the inline repository contract TipGenerator depends on,
// kept narrow so the service layer can be unit-tested with a mock.
type tipGeneratorRepo interface {
	CountActive(ctx context.Context, language, level string) (int, error)
	Create(ctx context.Context, tip *model.Tip) error
}

// tipGeneratorLLM is the inline LLM contract TipGenerator depends on.
type tipGeneratorLLM interface {
	GenerateTips(
		ctx context.Context,
		language, level string,
		category model.TipCategory,
		n int,
	) ([]external.GeneratedTip, error)
}

// TipGenerator fills the (language, level) tip bucket toward TipBucketTarget by
// calling the LLM, one category per cycle. It owns no transaction with session
// building — a failure here never affects session push.
type TipGenerator struct {
	tips  tipGeneratorRepo
	llm   tipGeneratorLLM
	model string // cfg.LLM.Model, recorded as the tip's source_model
}

// NewTipGenerator wires the generator with its repository, LLM client, and the
// configured model name used for source_model attribution.
func NewTipGenerator(tips tipGeneratorRepo, llm tipGeneratorLLM, model string) *TipGenerator {
	return &TipGenerator{tips: tips, llm: llm, model: model}
}

// newTipGeneratorFromClient adapts the shared external.LLMClient to the tip
// pipeline. GenerateTips is only on the concrete *DefaultLLMClient, so it asserts
// and, on failure, stores a true nil LLM (not a typed nil) so TopUpBucket's nil
// guard works as intended.
func newTipGeneratorFromClient(tips tipGeneratorRepo, client external.LLMClient, model string) *TipGenerator {
	concrete, ok := client.(*external.DefaultLLMClient)
	if !ok {
		return NewTipGenerator(tips, nil, model)
	}
	return NewTipGenerator(tips, concrete, model)
}

// TopUpBucket generates up to TipGeneratePerCycle new tips for the given
// (language, level) when the active balance is below TipBucketTarget. It is a
// no-op once the bucket is full, so callers can invoke it every cycle safely.
func (g *TipGenerator) TopUpBucket(ctx context.Context, language, level string) error {
	if g.llm == nil {
		return external.ErrAIConfigMissing
	}

	count, err := g.tips.CountActive(ctx, language, level)
	if err != nil {
		return err
	}
	if count >= TipBucketTarget {
		slog.InfoContext(ctx, "Tip bucket already full",
			"event", "tipgen.skip_full",
			"source", "service.tip_generator",
			"language", language,
			"level", level,
			"count", count,
		)
		return nil
	}

	category := pickTipCategory()

	generated, err := g.llm.GenerateTips(ctx, language, level, category, TipGeneratePerCycle)
	if err != nil {
		return err
	}

	promptVer := TipPromptVersion
	sourceModel := g.model

	var saved, skipped int
	for _, gt := range generated {
		tip := &model.Tip{
			Language:         language,
			ProficiencyLevel: level,
			Category:         category,
			Body:             gt.Body,
			SourceModel:      &sourceModel,
			SourcePromptVer:  &promptVer,
			IsActive:         true,
		}
		if err := g.tips.Create(ctx, tip); err != nil {
			skipped++
			slog.WarnContext(ctx, "Failed to persist generated tip",
				"event", "tipgen.create_failed",
				"source", "service.tip_generator",
				"language", language,
				"level", level,
				"category", category,
				"error", err,
			)
			continue
		}
		saved++
	}

	slog.InfoContext(ctx, "Tip bucket topped up",
		"event", "tipgen.topped_up",
		"source", "service.tip_generator",
		"language", language,
		"level", level,
		"category", category,
		"requested", TipGeneratePerCycle,
		"generated", saved,
		"skipped", skipped,
		"bucket_before", count,
	)
	return nil
}

// pickTipCategory selects a category uniformly at random. Round-robin would need
// last-used tracking (ADR-015 deemed it over-engineering); random spread across
// the 7 categories is expected to balance out as the bucket fills toward 50.
func pickTipCategory() model.TipCategory {
	categories := model.AllTipCategories()
	return categories[rand.Intn(len(categories))]
}
