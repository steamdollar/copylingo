package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/observability"
)

// userPushCron matches the 30-minute choices in each user's schedule settings.
const userPushCron = "*/30 * * * *"

type Scheduler struct {
	user       slotUsers
	session    slotSessions
	cron       *cron.Cron
	dispatcher *sessionDispatcher
}

type slotUsers interface {
	GetActiveTimezones(ctx context.Context) ([]string, error)
	GetUsersBySlot(
		ctx context.Context,
		slot model.SessionSlot,
		localTime string,
		timezone string,
	) ([]model.User, error)
}

type slotSessions interface {
	CountUnfinishedBatch(
		ctx context.Context,
		userIDs []int64,
	) (map[int64]int, error)
	dispatchSessions
}

type quizPusher interface {
	PushSession(
		ctx context.Context,
		chatID int64,
		sessionID int,
		sessionType string,
	) error
}

type studyPusher interface {
	PushSession(
		ctx context.Context,
		chatID int64,
		sessionID int,
	) error
}

type pushClaims interface {
	TryClaim(
		ctx context.Context,
		userID int64,
		slot model.SessionSlot,
		today string,
	) (bool, error)
}

type Dependencies struct {
	User        slotUsers
	Session     slotSessions
	QuizPusher  quizPusher
	StudyPusher studyPusher
	Cron        *cron.Cron
	Claims      pushClaims
}

func New(dependencies Dependencies) *Scheduler {
	scheduler := &Scheduler{
		user:    dependencies.User,
		session: dependencies.Session,
		cron:    dependencies.Cron,
	}
	scheduler.dispatcher = newSessionDispatcher(
		dependencies.Session,
		dependencies.QuizPusher,
		dependencies.StudyPusher,
		dependencies.Claims,
	)
	return scheduler
}

func (scheduler *Scheduler) Start() {
	if _, err := scheduler.cron.AddFunc(
		userPushCron,
		func() {
			scheduler.runJob(
				"dynamic_user_push",
				10*time.Minute,
				scheduler.tick,
			)
		},
	); err != nil {
		slog.Error(
			"Failed to register scheduler job",
			"event",
			"scheduler.job.registration_failed",
			"source",
			"scheduler",
			"job",
			"dynamic_user_push",
			"error",
			err,
		)
	} else {
		slog.Info(
			"Scheduler job registered",
			"event",
			"scheduler.job.registered",
			"source",
			"scheduler",
			"job",
			"dynamic_user_push",
			"cron",
			userPushCron,
		)
	}

	scheduler.cron.Start()
	slog.Info(
		"Scheduler started",
		"event",
		"scheduler.started",
		"source",
		"scheduler",
	)
}

// Stop stops new ticks and waits for a running dispatch (until ctx ends) before stopping the rate limiter:
// stopping it first would stall the dispatch after the daily push claim was taken.
func (scheduler *Scheduler) Stop(ctx context.Context) error {
	jobsDone := scheduler.cron.Stop()
	var waitErr error
	select {
	case <-jobsDone.Done():
	case <-ctx.Done():
		waitErr = fmt.Errorf(
			"wait for running scheduler job: %w",
			ctx.Err(),
		)
	}
	scheduler.dispatcher.limiter.Stop()
	slog.Info(
		"Scheduler stopped",
		"event",
		"scheduler.stopped",
		"source",
		"scheduler",
	)
	return waitErr
}

func (scheduler *Scheduler) runJob(
	name string,
	timeout time.Duration,
	run func(context.Context) error,
) {
	ctx := observability.WithAttributes(
		context.Background(),
		slog.String(
			"interaction_id",
			observability.NewInteractionID("job-"+name),
		),
		slog.String(
			"source",
			"scheduler",
		),
		slog.String(
			"job",
			name,
		),
	)
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(
			ctx,
			timeout,
		)
		defer cancel()
	}
	startedAt := time.Now()
	slog.InfoContext(
		ctx,
		"Scheduler job started",
		"event",
		"scheduler.job.started",
	)
	if err := run(ctx); err != nil {
		slog.ErrorContext(
			ctx,
			"Scheduler job failed",
			"event",
			"scheduler.job.failed",
			"duration_ms",
			time.Since(startedAt).Milliseconds(),
			"error",
			err,
		)
		return
	}
	slog.InfoContext(
		ctx,
		"Scheduler job completed",
		"event",
		"scheduler.job.completed",
		"duration_ms",
		time.Since(startedAt).Milliseconds(),
	)
}

func (scheduler *Scheduler) tick(ctx context.Context) error {
	timezones, err := scheduler.user.GetActiveTimezones(ctx)
	if err != nil {
		slog.WarnContext(
			ctx,
			"Failed to get active timezones, using Asia/Seoul fallback",
			"error",
			err,
		)
		timezones = []string{"Asia/Seoul"}
	}
	if len(timezones) == 0 {
		timezones = []string{"Asia/Seoul"}
	}

	slots := []model.SessionSlot{
		model.SessionSlotMorningStudy,
		model.SessionSlotMorningQuiz,
		model.SessionSlotEveningStudy,
		model.SessionSlotEveningQuiz,
	}

	var failures int

	for _, timezone := range timezones {
		location, err := time.LoadLocation(timezone)
		if err != nil {
			slog.WarnContext(
				ctx,
				"Failed to load timezone",
				"timezone",
				timezone,
				"error",
				err,
			)
			continue
		}

		nowLocal := time.Now().In(location)
		minuteSlot := (nowLocal.Minute() / 30) * 30
		localTime := fmt.Sprintf(
			"%02d:%02d",
			nowLocal.Hour(),
			minuteSlot,
		)

		for _, slot := range slots {
			users, err := scheduler.user.GetUsersBySlot(
				ctx,
				slot,
				localTime,
				timezone,
			)
			if err != nil {
				failures++
				slog.ErrorContext(
					ctx,
					"Failed to query users for slot",
					"event",
					"scheduler.slot.query_failed",
					"slot",
					slot,
					"timezone",
					timezone,
					"time",
					localTime,
					"error",
					err,
				)
				continue
			}
			if len(users) == 0 {
				continue
			}

			userIDs := make(
				[]int64,
				len(users),
			)
			for i, user := range users {
				userIDs[i] = user.ID
			}

			var unfinishedCounts map[int64]int
			counts, err := scheduler.session.CountUnfinishedBatch(
				ctx,
				userIDs,
			)
			if err != nil {
				slog.WarnContext(
					ctx,
					"Failed to batch count unfinished sessions",
					"error",
					err,
				)
			} else {
				unfinishedCounts = counts
			}

			if err := scheduler.dispatcher.dispatchBatch(
				ctx,
				slot,
				users,
				unfinishedCounts,
			); err != nil {
				failures++
			}
		}
	}

	if failures > 0 {
		return fmt.Errorf(
			"%d slot operations failed during tick",
			failures,
		)
	}
	return nil
}
