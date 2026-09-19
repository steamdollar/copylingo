package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/service"
)

type pushJob struct {
	user            model.User
	slot            model.SessionSlot
	unfinishedCount int
}

type sessionDispatcher struct {
	cfg      *config.Config
	services *service.Services
	bot      sessionPusher
	rdb      redis.Cmdable
	limiter  *rateLimiter
	workers  int
}

func newSessionDispatcher(
	cfg *config.Config,
	services *service.Services,
	bot sessionPusher,
	rdb redis.Cmdable,
) *sessionDispatcher {
	return &sessionDispatcher{
		cfg:      cfg,
		services: services,
		bot:      bot,
		rdb:      rdb,
		limiter:  newRateLimiter(25), // 25 msg/sec rate limit (Telegram safety margin)
		workers:  4,
	}
}

func (d *sessionDispatcher) dispatchBatch(
	ctx context.Context,
	slot model.SessionSlot,
	users []model.User,
	unfinishedCounts map[int64]int,
) error {
	if len(users) == 0 {
		return nil
	}

	jobs := make(chan pushJob, len(users))
	for _, u := range users {
		count := 0
		if unfinishedCounts != nil {
			count = unfinishedCounts[u.ID]
		}
		jobs <- pushJob{user: u, slot: slot, unfinishedCount: count}
	}
	close(jobs)

	numWorkers := d.workers
	if len(users) < numWorkers {
		numWorkers = len(users)
	}

	var wg sync.WaitGroup
	var errOnce sync.Once
	var firstErr error

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				if d.limiter != nil {
					if err := d.limiter.Wait(ctx); err != nil {
						return
					}
				}

				if err := d.dispatchUser(ctx, job.user, job.slot, job.unfinishedCount); err != nil {
					slog.ErrorContext(ctx, "Failed to dispatch user session",
						"event", "scheduler.dispatch.failed",
						"user_id", job.user.ID,
						"slot", job.slot,
						"error", err,
					)
					errOnce.Do(func() {
						firstErr = err
					})
				}
			}
		}()
	}

	wg.Wait()
	return firstErr
}

func (d *sessionDispatcher) dispatchUser(
	ctx context.Context,
	user model.User,
	slot model.SessionSlot,
	unfinishedCount int,
) error {
	// 1. Redis Idempotency Key check
	if d.rdb != nil {
		today := time.Now().Format("2006-01-02")
		lockKey := fmt.Sprintf("copylingo:push:lock:%d:%s:%s", user.ID, slot, today)
		acquired, err := d.rdb.SetNX(ctx, lockKey, "1", 24*time.Hour).Result()
		if err != nil {
			slog.WarnContext(ctx, "Redis idempotency lock check error; proceeding",
				"event", "scheduler.lock.error",
				"user_id", user.ID,
				"slot", slot,
				"error", err,
			)
		} else if !acquired {
			slog.InfoContext(ctx, "Session already pushed today for user; skipping",
				"event", "scheduler.lock.skipped",
				"user_id", user.ID,
				"slot", slot,
			)
			return nil
		}
	}

	// 2. Unfinished session backlog cap check (ADR-045)
	maxBacklog := 3
	if d.cfg != nil && d.cfg.Schedule.MaxUnfinishedSessions > 0 {
		maxBacklog = d.cfg.Schedule.MaxUnfinishedSessions
	}

	if unfinishedCount >= maxBacklog {
		reminded, err := d.remindUnfinishedSession(ctx, user.ID)
		if err != nil {
			return err
		}
		if reminded {
			return nil
		}
	}

	// 3. Build & push session
	switch slot {
	case model.SessionSlotMorningStudy:
		return d.buildAndPushStudy(ctx, user, service.StudyProfileMorning)
	case model.SessionSlotMorningQuiz:
		return d.buildAndPushQuiz(ctx, user, model.SessionMorning)
	case model.SessionSlotEveningStudy:
		return d.buildAndPushStudy(ctx, user, service.StudyProfileEvening)
	case model.SessionSlotEveningQuiz:
		return d.buildAndPushQuiz(ctx, user, model.SessionEvening)
	default:
		return fmt.Errorf("unsupported session slot: %s", slot)
	}
}

func (d *sessionDispatcher) buildAndPushStudy(
	ctx context.Context,
	user model.User,
	profile service.StudySessionProfile,
) error {
	if d.services == nil || d.services.StudySession == nil {
		return fmt.Errorf("study session service unavailable")
	}
	session, err := d.services.StudySession.BuildStudySessionWithProfile(
		ctx,
		user.ID,
		user.Language,
		user.ProficiencyLevel,
		profile,
	)
	if err != nil {
		return fmt.Errorf("build study session user_id=%d: %w", user.ID, err)
	}
	if session == nil {
		slog.WarnContext(ctx, "No study materials available for session",
			"event", "scheduler.study_session.empty",
			"user_id", user.ID,
		)
		return nil
	}

	if err := d.bot.PushStudySession(ctx, user.ID, session.ID); err != nil {
		return fmt.Errorf("push study session user_id=%d session_id=%d: %w", user.ID, session.ID, err)
	}

	slog.InfoContext(ctx, "Study session pushed",
		"event", "scheduler.study_session.pushed",
		"user_id", user.ID,
		"session_id", session.ID,
		"total_materials", session.TotalQuestions,
	)
	return nil
}

func (d *sessionDispatcher) buildAndPushQuiz(
	ctx context.Context,
	user model.User,
	sessionType model.SessionType,
) error {
	if d.services == nil || d.services.SessionBuilder == nil {
		return fmt.Errorf("session builder service unavailable")
	}

	var session *model.Session
	var err error

	switch sessionType {
	case model.SessionMorning:
		session, err = d.services.SessionBuilder.BuildMorningSession(
			ctx,
			user.ID,
			user.Language,
			user.ProficiencyLevel,
		)
	case model.SessionEvening:
		session, err = d.services.SessionBuilder.BuildEveningSession(
			ctx,
			user.ID,
			user.Language,
			user.ProficiencyLevel,
		)
	default:
		return fmt.Errorf("unsupported quiz session type: %s", sessionType)
	}

	if err != nil {
		return fmt.Errorf("build quiz session user_id=%d: %w", user.ID, err)
	}
	if session == nil {
		slog.WarnContext(ctx, "No questions available for session",
			"event", "scheduler.session.empty",
			"user_id", user.ID,
		)
		return nil
	}

	if err := d.bot.PushSession(ctx, user.ID, session.ID, string(sessionType)); err != nil {
		return fmt.Errorf("push quiz session user_id=%d session_id=%d: %w", user.ID, session.ID, err)
	}

	slog.InfoContext(ctx, "Session pushed",
		"event", "scheduler.session.pushed",
		"user_id", user.ID,
		"session_id", session.ID,
		"session_type", sessionType,
		"total_questions", session.TotalQuestions,
	)
	return nil
}

func (d *sessionDispatcher) remindUnfinishedSession(ctx context.Context, userID int64) (bool, error) {
	if d.services == nil || d.services.SessionQuery == nil {
		return false, fmt.Errorf("session query service unavailable")
	}
	if d.bot == nil {
		return false, fmt.Errorf("session pusher unavailable")
	}

	session, err := d.services.SessionQuery.GetOldestUnfinished(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("query unfinished session user_id=%d: %w", userID, err)
	}
	if session == nil {
		return false, nil
	}

	switch session.Mode {
	case model.SessionModeStudy:
		if err := d.bot.PushStudySession(ctx, userID, session.ID); err != nil {
			return true, fmt.Errorf("push study session reminder user_id=%d session_id=%d: %w", userID, session.ID, err)
		}
		slog.InfoContext(ctx, "Unfinished study session reminded",
			"event", "scheduler.study_session.reminded",
			"user_id", userID,
			"session_id", session.ID,
		)
		return true, nil
	case model.SessionModeQuiz, "":
		if err := d.bot.PushSession(ctx, userID, session.ID, string(session.Type)); err != nil {
			return true, fmt.Errorf("push quiz session reminder user_id=%d session_id=%d: %w", userID, session.ID, err)
		}
		slog.InfoContext(ctx, "Unfinished quiz session reminded",
			"event", "scheduler.session.reminded",
			"user_id", userID,
			"session_id", session.ID,
		)
		return true, nil
	default:
		return false, fmt.Errorf(
			"unsupported unfinished session mode user_id=%d session_id=%d mode=%q",
			userID,
			session.ID,
			session.Mode,
		)
	}
}

type rateLimiter struct {
	ticker *time.Ticker
}

func newRateLimiter(ratePerSec int) *rateLimiter {
	if ratePerSec <= 0 {
		ratePerSec = 25
	}
	interval := time.Second / time.Duration(ratePerSec)
	return &rateLimiter{ticker: time.NewTicker(interval)}
}

func (r *rateLimiter) Wait(ctx context.Context) error {
	if r == nil || r.ticker == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-r.ticker.C:
		return nil
	}
}

func (r *rateLimiter) Stop() {
	if r != nil && r.ticker != nil {
		r.ticker.Stop()
	}
}
