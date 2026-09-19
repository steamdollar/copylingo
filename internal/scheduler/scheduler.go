package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"

	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/observability"
	"github.com/lsj/copylingo/internal/pipeline"
	"github.com/lsj/copylingo/internal/service"
)

// Scheduler manages periodic tasks: content collection, session building, and pushing.
type Scheduler struct {
	cfg          *config.Config
	services     *service.Services
	bot          sessionPusher
	orchestrator *pipeline.Orchestrator
	cron         *cron.Cron
	rdb          redis.Cmdable
	dispatcher   *sessionDispatcher
}

type sessionPusher interface {
	PushSession(ctx context.Context, chatID int64, sessionID int, sessionType string) error
	PushStudySession(ctx context.Context, chatID int64, sessionID int) error
}

// New creates a new Scheduler.
func New(
	cfg *config.Config,
	services *service.Services,
	bot sessionPusher,
	orchestrator *pipeline.Orchestrator,
	c *cron.Cron,
	rdb ...redis.Cmdable,
) *Scheduler {
	var r redis.Cmdable
	if len(rdb) > 0 {
		r = rdb[0]
	}
	s := &Scheduler{
		cfg:          cfg,
		services:     services,
		bot:          bot,
		orchestrator: orchestrator,
		cron:         c,
		rdb:          r,
	}
	s.dispatcher = newSessionDispatcher(cfg, services, bot, r)
	return s
}

// Start registers all cron jobs and starts the scheduler.
func (s *Scheduler) Start() {
	// Content collection (daily at 03:00)
	if s.orchestrator != nil && !s.cfg.Schedule.ContentCollectCron.IsZero() {
		if _, err := s.cron.AddFunc(s.cfg.Schedule.ContentCollectCron.String(), func() {
			s.runJob("content_collection", 10*time.Minute, s.collectContent)
		}); err != nil {
			slog.Error("Failed to register scheduler job",
				"event", "scheduler.job.registration_failed",
				"source", "scheduler",
				"job", "content_collection",
				"error", err,
			)
		} else {
			slog.Info("Scheduler job registered",
				"event", "scheduler.job.registered",
				"source", "scheduler",
				"job", "content_collection",
				"cron", s.cfg.Schedule.ContentCollectCron.String(),
			)
		}
	}

	// Dynamic 30-minute interval user push (primary scheduling engine)
	if !s.cfg.Schedule.DynamicPushCron.IsZero() {
		if _, err := s.cron.AddFunc(s.cfg.Schedule.DynamicPushCron.String(), func() {
			s.runJob("dynamic_user_push", 10*time.Minute, s.tick)
		}); err != nil {
			slog.Error("Failed to register scheduler job",
				"event", "scheduler.job.registration_failed",
				"source", "scheduler",
				"job", "dynamic_user_push",
				"error", err,
			)
		} else {
			slog.Info("Scheduler job registered",
				"event", "scheduler.job.registered",
				"source", "scheduler",
				"job", "dynamic_user_push",
				"cron", s.cfg.Schedule.DynamicPushCron.String(),
			)
		}
	} else if s.cfg.Schedule.StudyPushCron.IsZero() && s.cfg.Schedule.AfternoonStudyPushCron.IsZero() &&
		s.cfg.Schedule.MorningPushCron.IsZero() && s.cfg.Schedule.EveningPushCron.IsZero() {
		// Default to dynamic 30-minute push if no schedule crons are configured
		defaultCron := "*/30 * * * *"
		if _, err := s.cron.AddFunc(defaultCron, func() {
			s.runJob("dynamic_user_push", 10*time.Minute, s.tick)
		}); err == nil {
			slog.Info("Scheduler job registered",
				"event", "scheduler.job.registered",
				"source", "scheduler",
				"job", "dynamic_user_push",
				"cron", defaultCron,
			)
		}
	}

	// Legacy study push cron (backward compatibility when explicitly configured)
	if !s.cfg.Schedule.StudyPushCron.IsZero() {
		if _, err := s.cron.AddFunc(s.cfg.Schedule.StudyPushCron.String(), func() {
			s.runJob("study_push", 0, func(ctx context.Context) error {
				return s.buildAndPushStudySessions(ctx, service.StudyProfileMorning)
			})
		}); err != nil {
			slog.Error("Failed to register scheduler job",
				"event", "scheduler.job.registration_failed",
				"source", "scheduler",
				"job", "study_push",
				"error", err,
			)
		}
	}

	// Legacy afternoon study push cron (backward compatibility when explicitly configured)
	if !s.cfg.Schedule.AfternoonStudyPushCron.IsZero() {
		if _, err := s.cron.AddFunc(s.cfg.Schedule.AfternoonStudyPushCron.String(), func() {
			s.runJob("afternoon_study_push", 0, func(ctx context.Context) error {
				return s.buildAndPushStudySessions(ctx, service.StudyProfileEvening)
			})
		}); err != nil {
			slog.Error("Failed to register scheduler job",
				"event", "scheduler.job.registration_failed",
				"source", "scheduler",
				"job", "afternoon_study_push",
				"error", err,
			)
		}
	}

	s.cron.Start()
	slog.Info("Scheduler started", "event", "scheduler.started", "source", "scheduler")
}

// Stop gracefully stops the scheduler.
func (s *Scheduler) Stop() {
	s.cron.Stop()
	if s.dispatcher != nil && s.dispatcher.limiter != nil {
		s.dispatcher.limiter.Stop()
	}
	slog.Info("Scheduler stopped", "event", "scheduler.stopped", "source", "scheduler")
}

func (s *Scheduler) collectContent(ctx context.Context) error {
	results := s.orchestrator.RunAll(ctx)
	var failures int
	for _, result := range results {
		if result.Err != nil {
			failures++
			slog.ErrorContext(ctx, "Content collection failed",
				"event", "scheduler.collection.failed",
				"fetcher", result.FetcherName,
				"error", result.Err,
			)
			continue
		}
		slog.InfoContext(ctx, "Content collection completed",
			"event", "scheduler.collection.completed",
			"fetcher", result.FetcherName,
			"saved", result.SaveResult.Saved,
			"duplicates", result.SaveResult.Duplicates,
		)
	}
	if failures > 0 {
		return fmt.Errorf("%d content collections failed", failures)
	}
	return nil
}

func (s *Scheduler) runJob(name string, timeout time.Duration, run func(context.Context) error) {
	ctx := observability.WithAttrs(context.Background(),
		slog.String("interaction_id", observability.NewInteractionID("job-"+name)),
		slog.String("source", "scheduler"),
		slog.String("job", name),
	)
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	startedAt := time.Now()
	slog.InfoContext(ctx, "Scheduler job started", "event", "scheduler.job.started")
	if err := run(ctx); err != nil {
		slog.ErrorContext(ctx, "Scheduler job failed",
			"event", "scheduler.job.failed",
			"duration_ms", time.Since(startedAt).Milliseconds(),
			"error", err,
		)
		return
	}
	slog.InfoContext(ctx, "Scheduler job completed",
		"event", "scheduler.job.completed",
		"duration_ms", time.Since(startedAt).Milliseconds(),
	)
}

// tick executes 30-minute interval dynamic dispatch across distinct timezones and slots.
func (s *Scheduler) tick(ctx context.Context) error {
	if s.services == nil || s.services.User == nil {
		return fmt.Errorf("user service unavailable")
	}

	timezones, err := s.services.User.GetActiveTimezones(ctx)
	if err != nil {
		slog.WarnContext(ctx, "Failed to get active timezones, using Asia/Seoul fallback", "error", err)
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

	var allUsers []model.User
	var failures int

	for _, tz := range timezones {
		loc, err := time.LoadLocation(tz)
		if err != nil {
			slog.WarnContext(ctx, "Failed to load timezone", "timezone", tz, "error", err)
			continue
		}

		nowLocal := time.Now().In(loc)
		minuteSlot := (nowLocal.Minute() / 30) * 30
		localTime := fmt.Sprintf("%02d:%02d", nowLocal.Hour(), minuteSlot)

		for _, slot := range slots {
			users, err := s.services.User.GetUsersBySlot(ctx, slot, localTime, tz)
			if err != nil {
				failures++
				slog.ErrorContext(ctx, "Failed to query users for slot",
					"event", "scheduler.slot.query_failed",
					"slot", slot,
					"timezone", tz,
					"time", localTime,
					"error", err,
				)
				continue
			}
			if len(users) == 0 {
				continue
			}

			allUsers = append(allUsers, users...)

			userIDs := make([]int64, len(users))
			for i, u := range users {
				userIDs[i] = u.ID
			}

			var unfinishedCounts map[int64]int
			if s.services.SessionQuery != nil {
				counts, err := s.services.SessionQuery.CountUnfinishedBatch(ctx, userIDs)
				if err != nil {
					slog.WarnContext(ctx, "Failed to batch count unfinished sessions", "error", err)
				} else {
					unfinishedCounts = counts
				}
			}

			if err := s.dispatcher.dispatchBatch(ctx, slot, users, unfinishedCounts); err != nil {
				failures++
			}
		}
	}

	if len(allUsers) > 0 {
		s.topUpTips(ctx, allUsers)
		s.topUpAudio(ctx, allUsers)
	}

	if failures > 0 {
		return fmt.Errorf("%d slot operations failed during tick", failures)
	}
	return nil
}

func (s *Scheduler) getDispatcher() *sessionDispatcher {
	if s.dispatcher == nil {
		s.dispatcher = newSessionDispatcher(s.cfg, s.services, s.bot, s.rdb)
	}
	return s.dispatcher
}

func (s *Scheduler) buildAndPushSessions(ctx context.Context, sessionType model.SessionType) error {
	if s.cfg == nil {
		return fmt.Errorf("scheduler config unavailable")
	}
	if s.services == nil || s.services.User == nil {
		return fmt.Errorf("user service unavailable")
	}
	users, err := s.services.User.GetAllUsers(ctx)
	if err != nil {
		return fmt.Errorf("get users: %w", err)
	}

	slot := model.SessionSlotMorningQuiz
	if sessionType == model.SessionEvening {
		slot = model.SessionSlotEveningQuiz
	}

	var failures int
	for _, user := range users {
		unfinishedCount := 0
		if s.services.SessionQuery != nil {
			count, err := s.services.SessionQuery.CountUnfinished(ctx, user.ID)
			if err != nil {
				failures++
				slog.ErrorContext(ctx, "Failed to count unfinished sessions",
					"event", "scheduler.session.count_failed",
					"user_id", user.ID,
					"error", err,
				)
				continue
			}
			unfinishedCount = count
		}

		if err := s.getDispatcher().dispatchUser(ctx, user, slot, unfinishedCount); err != nil {
			failures++
			slog.ErrorContext(ctx, "Failed to dispatch user session",
				"event", "scheduler.session.dispatch_failed",
				"user_id", user.ID,
				"error", err,
			)
		}
	}

	s.topUpTips(ctx, users)
	s.topUpAudio(ctx, users)

	if failures > 0 {
		return fmt.Errorf("%d session operations failed", failures)
	}
	return nil
}

func (s *Scheduler) buildAndPushStudySessions(ctx context.Context, profile service.StudySessionProfile) error {
	if s.cfg == nil {
		return fmt.Errorf("scheduler config unavailable")
	}
	if s.services == nil || s.services.User == nil {
		return fmt.Errorf("user service unavailable")
	}
	users, err := s.services.User.GetAllUsers(ctx)
	if err != nil {
		return fmt.Errorf("get users for study sessions: %w", err)
	}

	slot := model.SessionSlotMorningStudy
	if profile == service.StudyProfileEvening {
		slot = model.SessionSlotEveningStudy
	}

	var failures int
	for _, user := range users {
		unfinishedCount := 0
		if s.services.SessionQuery != nil {
			count, err := s.services.SessionQuery.CountUnfinished(ctx, user.ID)
			if err != nil {
				failures++
				slog.ErrorContext(ctx, "Failed to count unfinished sessions",
					"event", "scheduler.study_session.count_failed",
					"user_id", user.ID,
					"error", err,
				)
				continue
			}
			unfinishedCount = count
		}

		if err := s.getDispatcher().dispatchUser(ctx, user, slot, unfinishedCount); err != nil {
			failures++
			slog.ErrorContext(ctx, "Failed to dispatch user study session",
				"event", "scheduler.study_session.dispatch_failed",
				"user_id", user.ID,
				"error", err,
			)
		}
	}

	if failures > 0 {
		return fmt.Errorf("%d study session operations failed", failures)
	}
	return nil
}

func (s *Scheduler) remindUnfinishedSession(ctx context.Context, userID int64) (bool, error) {
	return s.getDispatcher().remindUnfinishedSession(ctx, userID)
}

// langLevelPair identifies a distinct (language, proficiency level) bucket.
type langLevelPair struct {
	Language string
	Level    string
}

// distinctLangLevelPairs collapses the user slice into the unique set of
// (language, level) pairs in a single pass.
func distinctLangLevelPairs(users []model.User) []langLevelPair {
	seen := make(map[langLevelPair]struct{}, len(users))
	pairs := make([]langLevelPair, 0, len(users))
	for _, user := range users {
		p := langLevelPair{Language: user.Language, Level: user.ProficiencyLevel}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		pairs = append(pairs, p)
	}
	return pairs
}

// topUpTips fills each distinct (language, level) tip bucket toward the target.
func (s *Scheduler) topUpTips(ctx context.Context, users []model.User) {
	if s.services == nil || s.services.TipGenerator == nil {
		return
	}
	for _, p := range distinctLangLevelPairs(users) {
		if err := s.services.TipGenerator.TopUpBucket(ctx, p.Language, p.Level); err != nil {
			slog.WarnContext(ctx, "Tip top-up failed",
				"event", "scheduler.tip.topup_failed",
				"language", p.Language,
				"level", p.Level,
				"error", err,
			)
			continue
		}
	}
}

// topUpAudio fills each distinct (language, level) bucket's missing listening clips.
func (s *Scheduler) topUpAudio(ctx context.Context, users []model.User) {
	if s.services == nil || s.services.Audio == nil {
		return
	}
	for _, p := range distinctLangLevelPairs(users) {
		if err := s.services.Audio.TopUpAudio(ctx, p.Language, p.Level); err != nil {
			slog.WarnContext(ctx, "Listening audio top-up failed",
				"event", "scheduler.audio.topup_failed",
				"language", p.Language,
				"level", p.Level,
				"error", err,
			)
			continue
		}
	}
}
