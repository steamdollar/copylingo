package scheduler

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/lsj/copylingo/internal/observability"
)

func TestRunJobInjectsCorrelationAndTimeout(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(observability.NewContextHandler(slog.NewJSONHandler(
		&output,
		nil,
	))))
	defer slog.SetDefault(previous)

	var interactionID string
	var hasDeadline bool
	scheduler := &Scheduler{}
	scheduler.runJob(
		"dynamic_user_push",
		time.Second,
		func(ctx context.Context) error {
			interactionID = observability.InteractionID(ctx)
			_, hasDeadline = ctx.Deadline()
			return nil
		},
	)

	if !strings.HasPrefix(
		interactionID,
		"job-dynamic_user_push-",
	) {
		t.Fatalf(
			"InteractionID() = %q, want job-dynamic_user_push prefix",
			interactionID,
		)
	}
	if !hasDeadline {
		t.Fatal("runJob() context has no deadline")
	}
	if !strings.Contains(
		output.String(),
		`"event":"scheduler.job.started"`,
	) {
		t.Fatalf(
			"start log missing: %s",
			output.String(),
		)
	}
	if !strings.Contains(
		output.String(),
		`"event":"scheduler.job.completed"`,
	) {
		t.Fatalf(
			"completion log missing: %s",
			output.String(),
		)
	}
	if !strings.Contains(
		output.String(),
		`"interaction_id":"`+interactionID+`"`,
	) {
		t.Fatalf(
			"correlation log missing: %s",
			output.String(),
		)
	}
}

func TestRunJobLogsFailure(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(observability.NewContextHandler(slog.NewJSONHandler(
		&output,
		nil,
	))))
	defer slog.SetDefault(previous)

	scheduler := &Scheduler{}
	scheduler.runJob(
		"dynamic_user_push",
		0,
		func(context.Context) error {
			return errors.New("push failed")
		},
	)

	if !strings.Contains(
		output.String(),
		`"event":"scheduler.job.failed"`,
	) {
		t.Fatalf(
			"failure log missing: %s",
			output.String(),
		)
	}
	if !strings.Contains(
		output.String(),
		`"error":"push failed"`,
	) {
		t.Fatalf(
			"failure reason missing: %s",
			output.String(),
		)
	}
}

func TestStartRegistersOnlyHalfHourlyUserPush(t *testing.T) {
	cronScheduler := cron.New()
	scheduler := New(Dependencies{
		Cron: cronScheduler,
	})

	scheduler.Start()
	defer scheduler.Stop(context.Background())

	// One aligned trigger serves all users and all four configurable slots.
	entries := cronScheduler.Entries()
	if len(entries) != 1 {
		t.Fatalf(
			"registered cron entries = %d, want 1",
			len(entries),
		)
	}
	start := time.Date(
		2026,
		time.September,
		26,
		8,
		1,
		0,
		0,
		time.UTC,
	)
	if got, want := entries[0].Schedule.Next(start), start.Add(29*time.Minute); !got.Equal(want) {
		t.Fatalf(
			"next user push = %s, want %s",
			got,
			want,
		)
	}
}

// onceSchedule fires once right after cron starts, then not again for a day.
type onceSchedule struct {
	fired atomic.Bool
}

func (schedule *onceSchedule) Next(now time.Time) time.Time {
	if schedule.fired.Swap(true) {
		return now.Add(24 * time.Hour)
	}
	return now.Add(time.Millisecond)
}

func TestStopWaitsForRunningJob(t *testing.T) {
	cronScheduler := cron.New()
	scheduler := New(Dependencies{Cron: cronScheduler})
	running := make(chan struct{})
	release := make(chan struct{})
	cronScheduler.Schedule(
		&onceSchedule{},
		cron.FuncJob(func() {
			close(running)
			<-release
		}),
	)
	cronScheduler.Start()
	select {
	case <-running:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the job to start")
	}

	stopErr := make(
		chan error,
		1,
	)
	go func() { stopErr <- scheduler.Stop(context.Background()) }()
	select {
	case err := <-stopErr:
		t.Fatalf(
			"Stop returned %v while a job was still running",
			err,
		)
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	select {
	case err := <-stopErr:
		if err != nil {
			t.Fatalf(
				"Stop error = %v, want nil once the job finished",
				err,
			)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not return after the job finished")
	}
}
