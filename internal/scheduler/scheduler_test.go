package scheduler

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/lsj/copylingo/internal/observability"
)

func TestRunJobInjectsCorrelationAndTimeout(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(observability.NewContextHandler(slog.NewJSONHandler(&output, nil))))
	defer slog.SetDefault(previous)

	var interactionID string
	var hasDeadline bool
	scheduler := &Scheduler{}
	scheduler.runJob("content_collection", time.Second, func(ctx context.Context) error {
		interactionID = observability.InteractionID(ctx)
		_, hasDeadline = ctx.Deadline()
		return nil
	})

	if !strings.HasPrefix(interactionID, "job-content_collection-") {
		t.Fatalf("InteractionID() = %q, want job-content_collection prefix", interactionID)
	}
	if !hasDeadline {
		t.Fatal("runJob() context has no deadline")
	}
	if !strings.Contains(output.String(), `"event":"scheduler.job.started"`) {
		t.Fatalf("start log missing: %s", output.String())
	}
	if !strings.Contains(output.String(), `"event":"scheduler.job.completed"`) {
		t.Fatalf("completion log missing: %s", output.String())
	}
	if !strings.Contains(output.String(), `"interaction_id":"`+interactionID+`"`) {
		t.Fatalf("correlation log missing: %s", output.String())
	}
}

func TestRunJobLogsFailure(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(observability.NewContextHandler(slog.NewJSONHandler(&output, nil))))
	defer slog.SetDefault(previous)

	scheduler := &Scheduler{}
	scheduler.runJob("morning_push", 0, func(context.Context) error {
		return errors.New("push failed")
	})

	if !strings.Contains(output.String(), `"event":"scheduler.job.failed"`) {
		t.Fatalf("failure log missing: %s", output.String())
	}
	if !strings.Contains(output.String(), `"error":"push failed"`) {
		t.Fatalf("failure reason missing: %s", output.String())
	}
}
