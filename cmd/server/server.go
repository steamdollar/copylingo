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
	"github.com/lsj/copylingo/internal/repository"
	"github.com/lsj/copylingo/internal/scheduler"
	"github.com/lsj/copylingo/internal/service"
)

func initApp(
	cfg *config.Config,
	db *sqlx.DB,
	rdb *redis.Client,
) (*service.Services, *bot.Bot, error) {
	repos := repository.NewRepositories(db)
	// Only the storage implementations know Redis commands and serialized keys.
	services := service.NewServices(
		repos,
		db,
		cfg,
		service.SessionStores{
			Quiz:  redisstore.NewQuizSessions(rdb),
			Study: redisstore.NewStudySessions(rdb),
		},
	)
	interactions := redisstore.NewInteractions(rdb)
	botHandler, err := bot.NewBot(
		cfg,
		services,
		bot.StateStores{
			Input: interactions, Drafts: interactions, Messages: interactions,
			Recovery: interactions, Timing: interactions,
		},
	)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"failed to initialize Telegram bot: %w",
			err,
		)
	}
	return services, botHandler, nil
}

func startWorkers(
	services *service.Services,
	botHandler *bot.Bot,
	rdb redis.Cmdable,
) func() {
	// Content collection has no scheduled job; keep its pipeline builder for future use.
	sched, stopSched := initScheduler(
		services,
		botHandler,
		rdb,
	)
	sched.Start()
	go botHandler.Start()
	go botHandler.RefreshStaleMiniAppMessages(context.Background())
	return stopSched
}

func initScheduler(
	services *service.Services,
	botHandler *bot.Bot,
	rdb redis.Cmdable,
) (*scheduler.Scheduler, func()) {
	cronScheduler := cron.New()
	sched := scheduler.New(
		services,
		botHandler,
		nil,
		cronScheduler,
		redisstore.NewPushClaims(rdb),
	)
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

func initPipeline(services *service.Services) *pipeline.Orchestrator {
	// NHK News Easy pipeline
	nhkClient := external.NewNHKClient()
	nhkFetcher := pipeline.NewNHKFetcher(nhkClient)
	processor := pipeline.NewPassThroughProcessor()
	saver := services.Content

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
