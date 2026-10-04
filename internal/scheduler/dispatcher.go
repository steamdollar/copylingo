package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/service"
)

const maxUnfinishedSessions = 3

type pushJob struct {
	user            model.User
	slot            model.SessionSlot
	unfinishedCount int
}

type sessionDispatcher struct {
	services *service.Services
	quiz     quizPusher
	study    studyPusher
	claims   pushClaims
	limiter  *rateLimiter
	workers  int
}

func newSessionDispatcher(
	services *service.Services,
	quiz quizPusher,
	study studyPusher,
	claims pushClaims,
) *sessionDispatcher {
	return &sessionDispatcher{
		services: services,
		quiz:     quiz,
		study:    study,
		claims:   claims,
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

	jobs := make(
		chan pushJob,
		len(users),
	)
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

				if err := d.dispatchUser(
					ctx,
					job.user,
					job.slot,
					job.unfinishedCount,
				); err != nil {
					slog.ErrorContext(
						ctx,
						"Failed to dispatch user session",
						"event",
						"scheduler.dispatch.failed",
						"user_id",
						job.user.ID,
						"slot",
						job.slot,
						"error",
						err,
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
	// Claim today's user/slot dispatch through the narrow scheduler storage contract.
	if d.claims != nil {
		today := time.Now().Format("2006-01-02")
		acquired, err := d.claims.TryClaim(
			ctx,
			user.ID,
			slot,
			today,
		)
		if err != nil {
			slog.WarnContext(
				ctx,
				"Redis idempotency lock check error; proceeding",
				"event",
				"scheduler.lock.error",
				"user_id",
				user.ID,
				"slot",
				slot,
				"error",
				err,
			)
		} else if !acquired {
			slog.InfoContext(
				ctx,
				"Session already pushed today for user; skipping",
				"event",
				"scheduler.lock.skipped",
				"user_id",
				user.ID,
				"slot",
				slot,
			)
			return nil
		}
	}

	// Each user with three unfinished sessions gets a reminder before another session is built.
	if unfinishedCount >= maxUnfinishedSessions {
		reminded, err := d.remindUnfinishedSession(
			ctx,
			user.ID,
		)
		if err != nil {
			return err
		}
		if reminded {
			return nil
		}
	}

	// 3. Build & push session
	if d.services == nil || d.services.Session == nil {
		return fmt.Errorf("session service unavailable")
	}
	session, err := d.services.Session.BuildForSlot(
		ctx,
		user,
		slot,
	)
	if err != nil {
		return fmt.Errorf(
			"build session user_id=%d slot=%s: %w",
			user.ID,
			slot,
			err,
		)
	}
	if session == nil {
		slog.WarnContext(
			ctx,
			"No content available for session slot",
			"event",
			"scheduler.session.empty",
			"user_id",
			user.ID,
			"slot",
			slot,
		)
		return nil
	}
	return d.pushBuiltSession(
		ctx,
		user.ID,
		session,
	)
}

// pushBuiltSession sends a freshly built session with the push matching its mode.
func (d *sessionDispatcher) pushBuiltSession(
	ctx context.Context,
	userID int64,
	session *model.Session,
) error {
	if session.Mode == model.SessionModeStudy {
		if err := d.study.PushSession(
			ctx,
			userID,
			session.ID,
		); err != nil {
			return fmt.Errorf(
				"push study session user_id=%d session_id=%d: %w",
				userID,
				session.ID,
				err,
			)
		}
		slog.InfoContext(
			ctx,
			"Study session pushed",
			"event",
			"scheduler.study_session.pushed",
			"user_id",
			userID,
			"session_id",
			session.ID,
			"total_materials",
			session.TotalQuestions,
		)
		return nil
	}

	if err := d.quiz.PushSession(
		ctx,
		userID,
		session.ID,
		string(session.Type),
	); err != nil {
		return fmt.Errorf(
			"push quiz session user_id=%d session_id=%d: %w",
			userID,
			session.ID,
			err,
		)
	}
	slog.InfoContext(
		ctx,
		"Session pushed",
		"event",
		"scheduler.session.pushed",
		"user_id",
		userID,
		"session_id",
		session.ID,
		"session_type",
		session.Type,
		"total_questions",
		session.TotalQuestions,
	)
	return nil
}

func (d *sessionDispatcher) remindUnfinishedSession(
	ctx context.Context,
	userID int64,
) (bool, error) {
	if d.services == nil || d.services.Session == nil {
		return false, fmt.Errorf("session service unavailable")
	}
	if d.quiz == nil || d.study == nil {
		return false, fmt.Errorf("session pusher unavailable")
	}

	session, err := d.services.Session.OldestUnfinished(
		ctx,
		userID,
	)
	if err != nil {
		return false, fmt.Errorf(
			"query unfinished session user_id=%d: %w",
			userID,
			err,
		)
	}
	if session == nil {
		return false, nil
	}

	switch session.Mode {
	case model.SessionModeStudy:
		if err := d.study.PushSession(
			ctx,
			userID,
			session.ID,
		); err != nil {
			return true, fmt.Errorf(
				"push study session reminder user_id=%d session_id=%d: %w",
				userID,
				session.ID,
				err,
			)
		}
		slog.InfoContext(
			ctx,
			"Unfinished study session reminded",
			"event",
			"scheduler.study_session.reminded",
			"user_id",
			userID,
			"session_id",
			session.ID,
		)
		return true, nil
	case model.SessionModeQuiz, "":
		if err := d.quiz.PushSession(
			ctx,
			userID,
			session.ID,
			string(session.Type),
		); err != nil {
			return true, fmt.Errorf(
				"push quiz session reminder user_id=%d session_id=%d: %w",
				userID,
				session.ID,
				err,
			)
		}
		slog.InfoContext(
			ctx,
			"Unfinished quiz session reminded",
			"event",
			"scheduler.session.reminded",
			"user_id",
			userID,
			"session_id",
			session.ID,
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
