package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/lsj/copylingo/internal/bot"
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
	bot          *bot.Bot
	orchestrator *pipeline.Orchestrator
	cron         *cron.Cron
}

// New creates a new Scheduler.
func New(
	cfg *config.Config,
	services *service.Services,
	bot *bot.Bot,
	orchestrator *pipeline.Orchestrator,
	c *cron.Cron,
) *Scheduler {
	return &Scheduler{
		cfg:          cfg,
		services:     services,
		bot:          bot,
		orchestrator: orchestrator,
		cron:         c,
	}
}

// Start registers all cron jobs and starts the scheduler.
func (s *Scheduler) Start() {
	// Content collection (daily at 03:00)
	if s.orchestrator != nil && s.cfg.Schedule.ContentCollectCron != "" {
		if _, err := s.cron.AddFunc(s.cfg.Schedule.ContentCollectCron, func() {
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
				"cron", s.cfg.Schedule.ContentCollectCron,
			)
		}
	}

	// Morning session: build and push
	if _, err := s.cron.AddFunc(s.cfg.Schedule.MorningPushCron, func() {
		s.runJob("morning_push", 0, func(ctx context.Context) error {
			return s.buildAndPushSessions(ctx, model.SessionMorning)
		})
	}); err != nil {
		slog.Error("Failed to register scheduler job",
			"event", "scheduler.job.registration_failed",
			"source", "scheduler",
			"job", "morning_push",
			"error", err,
		)
	} else {
		slog.Info("Scheduler job registered",
			"event", "scheduler.job.registered",
			"source", "scheduler",
			"job", "morning_push",
			"cron", s.cfg.Schedule.MorningPushCron,
		)
	}

	// Study session: build and push
	if _, err := s.cron.AddFunc(s.cfg.Schedule.StudyPushCron, func() {
		s.runJob("study_push", 0, s.buildAndPushStudySessions)
	}); err != nil {
		slog.Error("Failed to register scheduler job",
			"event", "scheduler.job.registration_failed",
			"source", "scheduler",
			"job", "study_push",
			"error", err,
		)
	} else {
		slog.Info("Scheduler job registered",
			"event", "scheduler.job.registered",
			"source", "scheduler",
			"job", "study_push",
			"cron", s.cfg.Schedule.StudyPushCron,
		)
	}

	// Evening session: build and push
	if _, err := s.cron.AddFunc(s.cfg.Schedule.EveningPushCron, func() {
		s.runJob("evening_push", 0, func(ctx context.Context) error {
			return s.buildAndPushSessions(ctx, model.SessionEvening)
		})
	}); err != nil {
		slog.Error("Failed to register scheduler job",
			"event", "scheduler.job.registration_failed",
			"source", "scheduler",
			"job", "evening_push",
			"error", err,
		)
	} else {
		slog.Info("Scheduler job registered",
			"event", "scheduler.job.registered",
			"source", "scheduler",
			"job", "evening_push",
			"cron", s.cfg.Schedule.EveningPushCron,
		)
	}

	s.cron.Start()
	slog.Info("Scheduler started", "event", "scheduler.started", "source", "scheduler")
}

// Stop gracefully stops the scheduler.
func (s *Scheduler) Stop() {
	s.cron.Stop()
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

func (s *Scheduler) buildAndPushSessions(ctx context.Context, sessionType model.SessionType) error {
	users, err := s.services.User.GetAllUsers(ctx)
	if err != nil {
		return fmt.Errorf("get users: %w", err)
	}

	// TODO: user가 많다고 가정했을때, 뭐 한 10만명 ~ 100만명 된다고 가정했을때, 이걸 빨리 할 방법이 있을까?
	// 예를 들어서, 세션 빌드 자체를 비동기로 하고, 세션이 준비되는대로 푸시를 한다거나? 아니면 세션 빌드와 푸시를 완전히 분리해서,
	// 세션 빌드는 큐에 넣고, 푸시는 큐에서 빼서 하는 식으로? 일단은 간단하게 동기적으로 처리하지만, 나중에 확장성을 고려해서 개선할 수 있을듯.
	var failures int
	for _, user := range users {
		var session *model.Session
		var err error

		switch sessionType {
		case model.SessionMorning:
			session, err = s.services.SessionBuilder.BuildMorningSession(
				ctx,
				user.ID,
				user.Language,
				user.ProficiencyLevel,
			)
		case model.SessionEvening:
			session, err = s.services.SessionBuilder.BuildEveningSession(
				ctx,
				user.ID,
				user.Language,
				user.ProficiencyLevel,
			)
		}

		if err != nil {
			failures++
			slog.ErrorContext(ctx, "Failed to build session",
				"event", "scheduler.session.build_failed",
				"user_id", user.ID,
				"session_type", sessionType,
				"error", err,
			)
			continue
		}

		if session == nil {
			slog.WarnContext(ctx, "No questions available for session",
				"event", "scheduler.session.empty",
				"user_id", user.ID,
				"session_type", sessionType,
			)
			continue
		}

		// Push via Telegram
		if err := s.bot.PushSession(ctx, user.ID, session.ID, string(sessionType)); err != nil {
			failures++
			slog.ErrorContext(ctx, "Failed to push session",
				"event", "scheduler.session.push_failed",
				"user_id", user.ID,
				"session_id", session.ID,
				"session_type", sessionType,
				"error", err,
			)
		} else {
			slog.InfoContext(ctx, "Session pushed",
				"event", "scheduler.session.pushed",
				"user_id", user.ID,
				"session_id", session.ID,
				"session_type", sessionType,
				"total_questions", session.TotalQuestions,
			)
		}
	}
	if failures > 0 {
		return fmt.Errorf("%d session operations failed", failures)
	}
	return nil
}

func (s *Scheduler) buildAndPushStudySessions(ctx context.Context) error {
	users, err := s.services.User.GetAllUsers(ctx)
	if err != nil {
		return fmt.Errorf("get users for study sessions: %w", err)
	}

	var failures int
	for _, user := range users {
		session, err := s.services.StudySession.BuildStudySession(ctx, user.ID, user.Language, user.ProficiencyLevel)
		if err != nil {
			failures++
			slog.ErrorContext(ctx, "Failed to build study session",
				"event", "scheduler.study_session.build_failed",
				"user_id", user.ID,
				"language", user.Language,
				"level", user.ProficiencyLevel,
				"error", err,
			)
			continue
		}

		if session == nil {
			slog.WarnContext(ctx, "No study materials available for session",
				"event", "scheduler.study_session.empty",
				"user_id", user.ID,
				"language", user.Language,
				"level", user.ProficiencyLevel,
			)
			continue
		}

		if err := s.bot.PushStudySession(ctx, user.ID, session.ID); err != nil {
			failures++
			slog.ErrorContext(ctx, "Failed to push study session",
				"event", "scheduler.study_session.push_failed",
				"user_id", user.ID,
				"session_id", session.ID,
				"error", err,
			)
			continue
		}

		slog.InfoContext(ctx, "Study session pushed",
			"event", "scheduler.study_session.pushed",
			"user_id", user.ID,
			"session_id", session.ID,
			"total_materials", session.TotalQuestions,
		)
	}
	if failures > 0 {
		return fmt.Errorf("%d study session operations failed", failures)
	}
	return nil
}
