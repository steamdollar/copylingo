package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/lsj/copylingo/internal/bot"
	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
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
func New(cfg *config.Config, services *service.Services, bot *bot.Bot, orchestrator *pipeline.Orchestrator, c *cron.Cron) *Scheduler {
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
		_, err := s.cron.AddFunc(s.cfg.Schedule.ContentCollectCron, func() {
			log.Println("[Scheduler] Starting content collection...")
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()

			results := s.orchestrator.RunAll(ctx)
			for _, r := range results {
				if r.Err != nil {
					log.Printf("[Scheduler] ERROR: collection %s failed: %v", r.FetcherName, r.Err)
				} else {
					log.Printf("[Scheduler] INFO: collection %s completed: saved=%d, duplicates=%d",
						r.FetcherName, r.SaveResult.Saved, r.SaveResult.Duplicates)
				}
			}
		})
		if err != nil {
			log.Printf("[Scheduler] ERROR: failed to register content collection cron: %v", err)
		} else {
			log.Printf("[Scheduler] Registered cron job: content_collection (%s)", s.cfg.Schedule.ContentCollectCron)
		}
	}

	// Morning session: build and push
	_, err := s.cron.AddFunc(s.cfg.Schedule.MorningPushCron, func() {
		log.Println("[Scheduler] Building and pushing morning sessions...")
		s.buildAndPushSessions(model.SessionMorning)
	})
	if err != nil {
		log.Printf("[Scheduler] ERROR: failed to register morning push cron: %v", err)
	} else {
		log.Printf("[Scheduler] Registered cron job: morning_push (%s)", s.cfg.Schedule.MorningPushCron)
	}

	// Evening session: build and push
	_, err = s.cron.AddFunc(s.cfg.Schedule.EveningPushCron, func() {
		log.Println("[Scheduler] Building and pushing evening sessions...")
		s.buildAndPushSessions(model.SessionEvening)
	})
	if err != nil {
		log.Printf("[Scheduler] ERROR: failed to register evening push cron: %v", err)
	} else {
		log.Printf("[Scheduler] Registered cron job: evening_push (%s)", s.cfg.Schedule.EveningPushCron)
	}

	s.cron.Start()
	log.Println("[Scheduler] Started and running")
}

// Stop gracefully stops the scheduler.
func (s *Scheduler) Stop() {
	s.cron.Stop()
	log.Println("[Scheduler] Stopped")
}

func (s *Scheduler) buildAndPushSessions(sessionType model.SessionType) {
	ctx := context.Background()

	users, err := s.services.User.GetAllUsers(ctx)
	if err != nil {
		log.Printf("[Scheduler] Error getting users: %v", err)
		return
	}

	// TODO: user가 많다고 가정했을때, 뭐 한 10만명 ~ 100만명 된다고 가정했을때, 이걸 빨리 할 방법이 있을까?
	// 예를 들어서, 세션 빌드 자체를 비동기로 하고, 세션이 준비되는대로 푸시를 한다거나? 아니면 세션 빌드와 푸시를 완전히 분리해서,
	// 세션 빌드는 큐에 넣고, 푸시는 큐에서 빼서 하는 식으로? 일단은 간단하게 동기적으로 처리하지만, 나중에 확장성을 고려해서 개선할 수 있을듯.
	for _, user := range users {
		var session *model.Session
		var err error

		switch sessionType {
		case model.SessionMorning:
			session, err = s.services.SessionBuilder.BuildMorningSession(ctx, user.ID, user.Language, user.ProficiencyLevel)
		case model.SessionEvening:
			session, err = s.services.SessionBuilder.BuildEveningSession(ctx, user.ID, user.Language, user.ProficiencyLevel)
		}

		if err != nil {
			log.Printf("[Scheduler] Error building session for user %d: %v", user.ID, err)
			continue
		}

		if session == nil {
			log.Printf("[Scheduler] No questions available for user %d", user.ID)
			continue
		}

		// Push via Telegram
		if err := s.bot.PushSession(ctx, user.ID, session.ID, string(sessionType)); err != nil {
			log.Printf("[Scheduler] Error pushing session to user %d: %v", user.ID, err)
		} else {
			log.Printf("[Scheduler] Pushed %s session to user %d (%d questions)",
				sessionType, user.ID, session.TotalQuestions)
		}
	}
}
