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

func initApp(cfg *config.Config) (_ *app, err error) {
	// db, redis setup
	db, rdb, err := initInfra(cfg)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to init infrastructure: %w",
			err,
		)
	}
	a := &app{closers: []io.Closer{db, rdb}}
	defer func() {
		if err != nil {
			a.Close()
		}
	}()

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
		return nil, fmt.Errorf(
			"failed to initialize Telegram bot: %w",
			err,
		)
	}
	a.bot = components.router
	a.refresher = components.sessionFlow

	// Content collection has no scheduled job (ADR-057).
	a.scheduler = scheduler.New(scheduler.Deps{
		User:        svc.user,
		Session:     svc.session,
		QuizPusher:  components.sessionFlow,
		StudyPusher: components.studyFlow,
		Cron:        cron.New(),
		Claims:      redisstore.NewPushClaims(rdb),
	})

	// The Mini App only reports a graded handwriting answer; SessionFlow
	// refreshes the Telegram message.
	miniappHandler := miniapp.NewHandler(miniapp.HandlerDeps{
		Session: svc.session,
		Verifier: miniapp.NewInitDataVerifier(
			cfg.Telegram.Token,
			miniapp.InitDataMaxAge,
		),
		HandwritingScreen: components.sessionFlow,
	})
	a.server = &http.Server{
		Addr: fmt.Sprintf(
			":%d",
			cfg.Server.Port,
		),
		Handler: setupRouter(
			cfg.Server.Mode == "release",
			healthHandler(
				db,
				rdb,
			),
			miniappHandler,
		),
	}
	return a, nil
}

// Run binds the HTTP port first so a port conflict starts nothing, then
// starts the scheduler, Telegram polling, the stale Mini App refresh and the
// HTTP server. It returns after ctx is cancelled or the HTTP server fails,
// once shutdown has drained in-flight work.
func (a *app) Run(ctx context.Context) error {
	listener, err := net.Listen(
		"tcp",
		a.server.Addr,
	)
	if err != nil {
		return fmt.Errorf(
			"HTTP server listen %s: %w",
			a.server.Addr,
			err,
		)
	}

	a.scheduler.Start()
	go a.bot.Start()
	go a.refresher.RefreshStaleMiniAppMessages(ctx)

	serveErr := make(
		chan error,
		1,
	)
	go func() {
		log.Printf(
			"HTTP server starting on port %d",
			listener.Addr().(*net.TCPAddr).Port,
		)
		serveErr <- a.server.Serve(listener)
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
	a.shutdown()
	return runErr
}

// shutdown stops taking Telegram updates, drains HTTP requests, then stops
// the scheduler. Each step waits for its in-flight work within one shared
// shutdownTimeout, so Close does not close DB/Redis under a running handler
// or job unless that timeout runs out. Connections stay open until Close.
func (a *app) shutdown() {
	log.Println("Shutting down...")

	ctx, cancel := context.WithTimeout(
		context.Background(),
		shutdownTimeout,
	)
	defer cancel()

	if err := a.bot.Stop(ctx); err != nil {
		log.Printf(
			"Telegram bot shutdown error: %v",
			err,
		)
	}
	if err := a.server.Shutdown(ctx); err != nil {
		log.Printf(
			"HTTP server shutdown error: %v",
			err,
		)
	}
	log.Println("Server stopped")

	if err := a.scheduler.Stop(ctx); err != nil {
		log.Printf(
			"Scheduler shutdown error: %v",
			err,
		)
	}
}

// Close closes the shared connections (DB, then Redis).
func (a *app) Close() {
	for _, closer := range a.closers {
		if err := closer.Close(); err != nil {
			log.Printf(
				"Connection close error: %v",
				err,
			)
		}
	}
}
