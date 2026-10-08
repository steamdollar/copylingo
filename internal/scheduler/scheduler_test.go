package scheduler

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/lsj/copylingo/internal/model"
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
	scheduler := New(Deps{
		Cron: cronScheduler,
	})

	scheduler.Start()
	defer scheduler.Stop()

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

// recordingTopUp records each (language, level) bucket it was asked to fill.
type recordingTopUp struct {
	buckets []string
}

func (r *recordingTopUp) record(
	language,
	level string,
) error {
	r.buckets = append(
		r.buckets,
		language+"/"+level,
	)
	return nil
}

func (r *recordingTopUp) TopUpBucket(
	_ context.Context,
	language,
	level string,
) error {
	return r.record(
		language,
		level,
	)
}

func (r *recordingTopUp) TopUpAudio(
	_ context.Context,
	language,
	level string,
) error {
	return r.record(
		language,
		level,
	)
}

func TestTopUpFillsEachDistinctBucketAndSkipsUnsetAudio(t *testing.T) {
	users := []model.User{
		{ID: 1, Language: "ja", ProficiencyLevel: "N5"},
		{ID: 2, Language: "ja", ProficiencyLevel: "N5"},
		{ID: 3, Language: "ja", ProficiencyLevel: "N4"},
	}
	want := "ja/N5,ja/N4"

	tips := &recordingTopUp{}
	audio := &recordingTopUp{}
	withAudio := New(Deps{
		Tip:   tips,
		Audio: audio,
		Cron:  cron.New(),
	})
	withAudio.topUpTips(
		context.Background(),
		users,
	)
	withAudio.topUpAudio(
		context.Background(),
		users,
	)
	if got := strings.Join(
		tips.buckets,
		",",
	); got != want {
		t.Fatalf(
			"tip buckets = %s, want %s",
			got,
			want,
		)
	}
	if got := strings.Join(
		audio.buckets,
		",",
	); got != want {
		t.Fatalf(
			"audio buckets = %s, want %s",
			got,
			want,
		)
	}

	// Without a TTS key cmd/server leaves Audio unset; the top-up is a no-op.
	withoutAudio := New(Deps{
		Tip:  &recordingTopUp{},
		Cron: cron.New(),
	})
	withoutAudio.topUpAudio(
		context.Background(),
		users,
	)
}
