package main

import (
	"fmt"
	"log"

	_ "github.com/lib/pq"

	"github.com/lsj/copylingo/internal/bot"
	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/repository"
	"github.com/lsj/copylingo/internal/service"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("Application terminated with error: %v", err)
	}
}

func run() error {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Connect to PostgreSQL
	db, err := initDB(cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Connect to Redis
	rdb, err := initRedis(cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}
	defer rdb.Close()

	// Initialize repositories and services
	repos := repository.NewRepositories(db)
	services := service.NewServices(repos, cfg)

	// Initialize Telegram bot
	botHandler, err := bot.New(cfg, services, rdb)
	if err != nil {
		return fmt.Errorf("failed to initialize Telegram bot: %w", err)
	}

	// Initialize content collection pipeline
	// data fetcher
	orchestrator := initPipeline(repos)

	// Initialize and start scheduler
	sched, stopSched := initScheduler(cfg, services, botHandler, orchestrator)
	sched.Start()
	defer stopSched()

	go botHandler.Start()

	// Start HTTP server (health check + admin API)
	router := setupRouter(cfg, db, rdb)
	srv := startHTTPServer(cfg, router)

	// Graceful shutdown
	waitForShutdown(srv, botHandler)

	return nil
}
