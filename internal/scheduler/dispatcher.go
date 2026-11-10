package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/lsj/copylingo/internal/model"
)

const maxUnfinishedSessions = 3

type pushJob struct {
	user            model.User
	slot            model.SessionSlot
	unfinishedCount int
}

type dispatchSessions interface {
	BuildForSlot(
		ctx context.Context,
		user model.User,
		slot model.SessionSlot,
	) (*model.Session, error)
	OldestUnfinished(
		ctx context.Context,
		userID int64,
	) (*model.Session, error)
}

type sessionDispatcher struct {
	session dispatchSessions
	quiz    quizPusher
	study   studyPusher
	claims  pushClaims
	limiter *rateLimiter
	workers int
}

func newSessionDispatcher(
	session dispatchSessions,
	quiz quizPusher,
	study studyPusher,
	claims pushClaims,
) *sessionDispatcher {
	return &sessionDispatcher{
		session: session,
		quiz:    quiz,
		study:   study,
		claims:  claims,
		limiter: newRateLimiter(25), // msg/sec; Telegram safety margin
		workers: 4,
	}
}

func (dispatcher *sessionDispatcher) dispatchBatch(
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
	for _, user := range users {
		count := 0
		if unfinishedCounts != nil {
			count = unfinishedCounts[user.ID]
		}
		jobs <- pushJob{user: user, slot: slot, unfinishedCount: count}
	}
	close(jobs)

	workerCount := dispatcher.workers
	if len(users) < workerCount {
		workerCount = len(users)
	}

	var wg sync.WaitGroup
	var errOnce sync.Once
	var firstErr error

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				if err := dispatcher.limiter.Wait(ctx); err != nil {
					return
				}

				if err := dispatcher.dispatchUser(
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

func (dispatcher *sessionDispatcher) dispatchUser(
	ctx context.Context,
	user model.User,
	slot model.SessionSlot,
	unfinishedCount int,
) error {
	// Claim today's user/slot dispatch through the narrow scheduler storage contract.
	today := time.Now().Format("2006-01-02")
	acquired, err := dispatcher.claims.TryClaim(
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

	// Each user with three unfinished sessions gets a reminder before another session is built.
	if unfinishedCount >= maxUnfinishedSessions {
		reminded, err := dispatcher.remindUnfinishedSession(
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

	session, err := dispatcher.session.BuildForSlot(
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
	return dispatcher.pushBuiltSession(
		ctx,
		user.ID,
		session,
	)
}

func (dispatcher *sessionDispatcher) pushBuiltSession(
	ctx context.Context,
	userID int64,
	session *model.Session,
) error {
	if session.Mode == model.SessionModeStudy {
		if err := dispatcher.study.PushSession(
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

	if err := dispatcher.quiz.PushSession(
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

func (dispatcher *sessionDispatcher) remindUnfinishedSession(
	ctx context.Context,
	userID int64,
) (bool, error) {
	session, err := dispatcher.session.OldestUnfinished(
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
		if err := dispatcher.study.PushSession(
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
		if err := dispatcher.quiz.PushSession(
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

func newRateLimiter(ratePerSecond int) *rateLimiter {
	interval := time.Second / time.Duration(ratePerSecond)
	return &rateLimiter{ticker: time.NewTicker(interval)}
}

func (limiter *rateLimiter) Wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-limiter.ticker.C:
		return nil
	}
}

func (limiter *rateLimiter) Stop() {
	limiter.ticker.Stop()
}
