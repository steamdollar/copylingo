package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/lsj/copylingo/internal/bot"
	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/miniapp"
	"github.com/lsj/copylingo/internal/redisstore"
	"github.com/lsj/copylingo/internal/scheduler"
)

const shutdownTimeout = 10 * time.Second

type app struct {
	bot       *bot.Bot
	scheduler *scheduler.Scheduler
	refresher *bot.SessionFlow
	server    *http.Server
	closers   []io.Closer // closed in order by Close
}

func initializeApp(appConfig *config.Config) (_ *app, err error) {
	// db, redis setup
	db, redisClient, err := initializeInfrastructure(appConfig)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to init infrastructure: %w",
			err,
		)
	}
	application := &app{closers: []io.Closer{db, redisClient}}
	defer func() {
		if err != nil {
			application.Close()
		}
	}()

	services := newServices(
		appConfig,
		db,
		redisClient,
	)

	components, err := initializeBot(
		appConfig,
		services,
		redisstore.NewInteractions(redisClient),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to initialize Telegram bot: %w",
			err,
		)
	}
	application.bot = components.router
	application.refresher = components.sessionFlow

	// Content collection has no scheduled job (ADR-057).
	application.scheduler = scheduler.New(scheduler.Dependencies{
		User:        services.user,
		Session:     services.session,
		QuizPusher:  components.sessionFlow,
		StudyPusher: components.studyFlow,
		Cron:        cron.New(),
		Claims:      redisstore.NewPushClaims(redisClient),
	})

	// The Mini App only reports a graded handwriting answer; SessionFlow
	// refreshes the Telegram message.
	miniappHandler := miniapp.NewHandler(miniapp.HandlerDependencies{
		Session: services.session,
		Verifier: miniapp.NewInitDataVerifier(
			appConfig.Telegram.Token,
			miniapp.InitDataMaxAge,
		),
		HandwritingScreen: components.sessionFlow,
	})
	application.server = &http.Server{
		Addr: fmt.Sprintf(
			":%d",
			appConfig.Server.Port,
		),
		Handler: setupRouter(
			appConfig.Server.Mode == "release",
			healthHandler(
				db,
				redisClient,
			),
			miniappHandler,
		),
	}
	return application, nil
}

// Run binds the HTTP port first so a port conflict starts nothing, then
// starts the scheduler, Telegram polling, the stale Mini App refresh and the
// HTTP server. It returns after ctx is cancelled or the HTTP server fails,
// once shutdown has drained in-flight work.
func (application *app) Run(ctx context.Context) error {
	listener, err := net.Listen(
		"tcp",
		application.server.Addr,
	)
	if err != nil {
		return fmt.Errorf(
			"HTTP server listen %s: %w",
			application.server.Addr,
			err,
		)
	}

	application.scheduler.Start()
	go application.bot.Start()
	go application.refresher.RefreshStaleMiniAppMessages(ctx)

	serveErr := make(
		chan error,
		1,
	)
	go func() {
		log.Printf(
			"HTTP server starting on port %d",
			listener.Addr().(*net.TCPAddr).Port,
		)
		serveErr <- application.server.Serve(listener)
	}()

	var runErr error
	select {
	case <-ctx.Done():
	case err := <-serveErr:
		// Shutdown has not run yet, so any return from Serve is a failure.
		runErr = fmt.Errorf(
			"HTTP server error: %w",
			err,
		)
	}
	application.shutdown()
	return runErr
}

// shutdown stops taking Telegram updates, drains HTTP requests, then stops
// the scheduler. Each step waits for its in-flight work within one shared
// shutdownTimeout, so Close does not close DB/Redis under a running handler
// or job unless that timeout runs out. Connections stay open until Close.
func (application *app) shutdown() {
	log.Println("Shutting down...")

	ctx, cancel := context.WithTimeout(
		context.Background(),
		shutdownTimeout,
	)
	defer cancel()

	if err := application.bot.Stop(ctx); err != nil {
		log.Printf(
			"Telegram bot shutdown error: %v",
			err,
		)
	}
	if err := application.server.Shutdown(ctx); err != nil {
		log.Printf(
			"HTTP server shutdown error: %v",
			err,
		)
	}
	log.Println("Server stopped")

	if err := application.scheduler.Stop(ctx); err != nil {
		log.Printf(
			"Scheduler shutdown error: %v",
			err,
		)
	}
}

// Close closes the shared connections (DB, then Redis).
func (application *app) Close() {
	for _, closer := range application.closers {
		if err := closer.Close(); err != nil {
			log.Printf(
				"Connection close error: %v",
				err,
			)
		}
	}
}
