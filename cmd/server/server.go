package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"

	"github.com/lsj/copylingo/internal/bot"
	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/external"
	"github.com/lsj/copylingo/internal/pipeline"
	"github.com/lsj/copylingo/internal/redisstore"
	"github.com/lsj/copylingo/internal/scheduler"
	"github.com/lsj/copylingo/internal/service"
)

// botComponents is the Telegram side initApp assembles. It stays inside
// cmd/server: each consumer receives only the flow it calls (ADR-059 §8.3).
type botComponents struct {
	router      *bot.Bot
	sessionFlow *bot.SessionFlow
	studyFlow   *bot.StudyFlow
}

func initApp(
	cfg *config.Config,
	db *sqlx.DB,
	rdb *redis.Client,
) (services, botComponents, error) {
	svc := newServices(
		cfg,
		db,
		rdb,
	)
	components, err := initBot(
		cfg,
		svc,
		redisstore.NewInteractions(rdb),
	)
	if err != nil {
		return services{}, botComponents{}, fmt.Errorf(
			"failed to initialize Telegram bot: %w",
			err,
		)
	}
	return svc, components, nil
}

// initBot creates one Telegram client, each feature flow over the services
// and Redis interaction state it uses, and the router over the flows.
func initBot(
	cfg *config.Config,
	svc services,
	interactions *redisstore.Interactions,
) (botComponents, error) {
	telegram, err := bot.NewTelegramClient(
		cfg.Telegram.Token,
		cfg.Telegram.Debug,
	)
	if err != nil {
		return botComponents{}, err
	}

	studyFlow := bot.NewStudyFlow(bot.StudyFlowDeps{
		Telegram:           telegram,
		Session:            svc.session,
		MaterialPreference: svc.materialPreference,
		Input:              interactions,
	})
	sessionDeps := bot.SessionFlowDeps{
		Telegram:           telegram,
		Session:            svc.session,
		User:               svc.user,
		MaterialPreference: svc.materialPreference,
		Input:              interactions,
		Drafts:             interactions,
		Messages:           interactions,
		Recovery:           interactions,
		Timing:             interactions,
		Study:              studyFlow,
		PublicBaseURL:      cfg.Server.PublicBaseURL,
	}
	// Audio is nil without a TTS key; assigning a nil *AudioService would make
	// a non-nil interface and bypass SessionFlow's "audio unavailable" path.
	if svc.audio != nil {
		sessionDeps.Audio = svc.audio
	}
	sessionFlow := bot.NewSessionFlow(sessionDeps)

	router := bot.NewBot(bot.BotDeps{
		Telegram:    telegram,
		User:        svc.user,
		Session:     svc.session,
		Analyzer:    svc.analyzer,
		Input:       interactions,
		SessionFlow: sessionFlow,
		StudyFlow:   studyFlow,
		SettingsFlow: bot.NewSettingsFlow(bot.SettingsFlowDeps{
			Telegram:           telegram,
			User:               svc.user,
			MaterialPreference: svc.materialPreference,
		}),
		LLMQuestionFlow: bot.NewLLMQuestionFlow(bot.LLMQuestionFlowDeps{
			Telegram:    telegram,
			User:        svc.user,
			LLMQuestion: svc.llmQuestion,
			Session:     svc.session,
			Input:       interactions,
		}),
	})
	return botComponents{
		router:      router,
		sessionFlow: sessionFlow,
		studyFlow:   studyFlow,
	}, nil
}

func startWorkers(
	svc services,
	components botComponents,
	rdb redis.Cmdable,
) func() {
	// Content collection has no scheduled job; keep its pipeline builder for future use.
	sched, stopSched := initScheduler(
		svc,
		components,
		rdb,
	)
	sched.Start()
	go components.router.Start()
	go components.sessionFlow.RefreshStaleMiniAppMessages(context.Background())
	return stopSched
}

func initScheduler(
	svc services,
	components botComponents,
	rdb redis.Cmdable,
) (*scheduler.Scheduler, func()) {
	schedDeps := scheduler.Deps{
		User:        svc.user,
		Session:     svc.session,
		Tip:         svc.tip,
		QuizPusher:  components.sessionFlow,
		StudyPusher: components.studyFlow,
		Cron:        cron.New(),
		Claims:      redisstore.NewPushClaims(rdb),
	}
	// Same typed-nil guard as SessionFlowDeps.Audio: without a TTS key the
	// scheduler must see a nil interface and skip the audio top-up.
	if svc.audio != nil {
		schedDeps.Audio = svc.audio
	}
	sched := scheduler.New(schedDeps)
	return sched, func() { sched.Stop() }
}

func startHTTPServer(
	cfg *config.Config,
	router http.Handler,
) *http.Server {
	srv := &http.Server{
		Addr: fmt.Sprintf(
			":%d",
			cfg.Server.Port,
		),
		Handler: router,
	}

	go func() {
		log.Printf(
			"HTTP server starting on port %d",
			cfg.Server.Port,
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf(
				"HTTP server error: %v",
				err,
			)
		}
	}()

	return srv
}

// initPipeline is kept for re-enabling content collection (ADR-057): startup
// does not build it, and a caller creates ContentService on that path.
func initPipeline(content *service.ContentService) *pipeline.Orchestrator {
	// NHK News Easy pipeline
	nhkClient := external.NewNHKClient()
	nhkFetcher := pipeline.NewNHKFetcher(nhkClient)
	processor := pipeline.NewPassThroughProcessor()
	saver := content

	orchestrator := pipeline.NewOrchestrator()
	orchestrator.Register(
		nhkFetcher,
		processor,
		saver,
	)

	log.Println("Content collection pipeline initialized")
	return orchestrator
}

func waitForShutdown(
	srv *http.Server,
	botHandler *bot.Bot,
) {
	quit := make(
		chan os.Signal,
		1,
	)
	signal.Notify(
		quit,
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	<-quit

	log.Println("Shutting down...")

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	botHandler.Stop()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf(
			"HTTP server shutdown error: %v",
			err,
		)
	}

	log.Println("Server stopped")
}
