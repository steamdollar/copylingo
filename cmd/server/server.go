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

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"

	"github.com/lsj/copylingo/internal/bot"
	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/external"
	"github.com/lsj/copylingo/internal/miniapp"
	"github.com/lsj/copylingo/internal/pipeline"
	"github.com/lsj/copylingo/internal/repository"
	"github.com/lsj/copylingo/internal/scheduler"
	"github.com/lsj/copylingo/internal/service"
)

func initInfra(cfg *config.Config) (*sqlx.DB, *redis.Client, func(), error) {
	db, err := initDB(cfg)
	if err != nil {
		return nil, nil, nil, err
	}
	rdb, err := initRedis(cfg)
	if err != nil {
		db.Close()
		return nil, nil, nil, err
	}
	return db, rdb, func() { db.Close(); rdb.Close() }, nil
}

func initApp(cfg *config.Config, db *sqlx.DB, rdb *redis.Client) (*repository.Repositories, *service.Services, *bot.Bot, error) {
	repos := repository.NewRepositories(db)
	services := service.NewServices(repos, cfg, rdb)
	botHandler, err := bot.New(cfg, services, rdb)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to initialize Telegram bot: %w", err)
	}
	return repos, services, botHandler, nil
}

func startWorkers(cfg *config.Config, services *service.Services, botHandler *bot.Bot, repos *repository.Repositories) func() {
	orchestrator := initPipeline(repos)
	sched, stopSched := initScheduler(cfg, services, botHandler, orchestrator)
	sched.Start()
	go botHandler.Start()
	go botHandler.RefreshStaleMiniAppMessages(context.Background())
	return stopSched
}

func initDB(cfg *config.Config) (*sqlx.DB, error) {
	db, err := sqlx.Connect("postgres", cfg.DB.DSN())
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	log.Println("Connected to PostgreSQL")
	return db, nil
}

func initRedis(cfg *config.Config) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	log.Println("Connected to Redis")
	return rdb, nil
}

func initPipeline(repos *repository.Repositories) *pipeline.Orchestrator {
	// NHK News Easy pipeline
	nhkClient := external.NewNHKClient()
	nhkFetcher := pipeline.NewNHKFetcher(nhkClient)
	processor := pipeline.NewPassThroughProcessor()
	saver := pipeline.NewContentSaver(repos.Content)

	orchestrator := pipeline.NewOrchestrator()
	orchestrator.Register(nhkFetcher, processor, saver)

	log.Println("Content collection pipeline initialized")
	return orchestrator
}

func initScheduler(cfg *config.Config, services *service.Services, botHandler *bot.Bot, orchestrator *pipeline.Orchestrator) (*scheduler.Scheduler, func()) {
	cronScheduler := cron.New()
	sched := scheduler.New(cfg, services, botHandler, orchestrator, cronScheduler)
	return sched, func() { sched.Stop() }
}

func startHTTPServer(cfg *config.Config, router http.Handler) *http.Server {
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: router,
	}

	go func() {
		log.Printf("HTTP server starting on port %d", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	return srv
}

func waitForShutdown(srv *http.Server, botHandler *bot.Bot) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	botHandler.Stop()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	log.Println("Server stopped")
}

func setupRouter(cfg *config.Config, db *sqlx.DB, rdb *redis.Client, services *service.Services, botHandler *bot.Bot) *gin.Engine {
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(requestLoggingMiddleware(), structuredRecoveryMiddleware())

	// Health check
	r.GET("/health", func(c *gin.Context) {
		// Check DB
		if err := db.Ping(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "unhealthy",
				"error":  "database connection failed",
			})
			return
		}

		// Check Redis
		if err := rdb.Ping(c.Request.Context()).Err(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "unhealthy",
				"error":  "redis connection failed",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
			"time":   time.Now().Format(time.RFC3339),
		})
	})

	miniapp.RegisterRoutes(r, cfg, services, rdb, botHandler)

	return r
}
