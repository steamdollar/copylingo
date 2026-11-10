package pipeline

import (
	"context"
	"fmt"
	"log"
)

// Pipeline represents a complete fetch-process-save workflow.
type Pipeline struct {
	Fetcher   Fetcher
	Processor Processor
	Saver     Saver
}

// Orchestrator manages multiple pipelines and coordinates their execution.
type Orchestrator struct {
	pipelines []Pipeline
}

// NewOrchestrator creates a new pipeline orchestrator.
func NewOrchestrator() *Orchestrator {
	return &Orchestrator{
		pipelines: make(
			[]Pipeline,
			0,
		),
	}
}

// Register adds a pipeline to the orchestrator.
func (orchestrator *Orchestrator) Register(
	fetcher Fetcher,
	processor Processor,
	saver Saver,
) {
	orchestrator.pipelines = append(
		orchestrator.pipelines,
		Pipeline{
			Fetcher:   fetcher,
			Processor: processor,
			Saver:     saver,
		},
	)
	log.Printf(
		"[Orchestrator] INFO: registered pipeline: %s",
		fetcher.Name(),
	)
}

// RunAll executes all registered pipelines sequentially.
// Individual pipeline failures don't stop other pipelines.
func (orchestrator *Orchestrator) RunAll(ctx context.Context) []PipelineResult {
	results := make(
		[]PipelineResult,
		0,
		len(orchestrator.pipelines),
	)

	for _, pipeline := range orchestrator.pipelines {
		result := orchestrator.runPipeline(
			ctx,
			pipeline,
		)
		results = append(
			results,
			result,
		)

		if result.Err != nil {
			log.Printf(
				"[Orchestrator] ERROR: pipeline %s failed: %v",
				result.FetcherName,
				result.Err,
			)
		} else {
			log.Printf(
				"[Orchestrator] INFO: pipeline %s completed: fetched=%d, saved=%d, duplicates=%d",
				result.FetcherName,
				result.FetchedCount,
				result.SaveResult.Saved,
				result.SaveResult.Duplicates,
			)
		}
	}

	return results
}

func (orchestrator *Orchestrator) runPipeline(
	ctx context.Context,
	pipeline Pipeline,
) PipelineResult {
	result := PipelineResult{
		FetcherName: pipeline.Fetcher.Name(),
	}

	// Fetch
	rawContents, err := pipeline.Fetcher.Fetch(ctx)
	if err != nil {
		result.Err = fmt.Errorf(
			"fetch: %w",
			err,
		)
		return result
	}
	result.FetchedCount = len(rawContents)

	if len(rawContents) == 0 {
		log.Printf(
			"[Orchestrator] INFO: pipeline %s: no content fetched",
			pipeline.Fetcher.Name(),
		)
		return result
	}

	// Process
	contents, err := pipeline.Processor.Process(
		ctx,
		rawContents,
	)
	if err != nil {
		result.Err = fmt.Errorf(
			"process: %w",
			err,
		)
		return result
	}

	// Save
	saveResult, err := pipeline.Saver.Save(
		ctx,
		contents,
	)
	if err != nil {
		result.Err = fmt.Errorf(
			"save: %w",
			err,
		)
		return result
	}
	result.SaveResult = saveResult

	return result
}

// PipelineCount returns the number of registered pipelines.
func (orchestrator *Orchestrator) PipelineCount() int {
	return len(orchestrator.pipelines)
}
