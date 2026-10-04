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

	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/miniapp"
	"github.com/lsj/copylingo/internal/redisstore"
	"github.com/lsj/copylingo/internal/scheduler"
)

// shutdownTimeout bounds how long in-flight HTTP requests may finish.
const shutdownTimeout = 10 * time.Second

// updatePoller is the Telegram update loop: Start blocks until Stop.
type updatePoller interface {
	Start()
	Stop()
}

// jobScheduler runs the periodic user-session dispatch.
type jobScheduler interface {
	Start()
	Stop()
}

// miniAppRefresher re-renders Mini App messages a restart left stale.
type miniAppRefresher interface {
	RefreshStaleMiniAppMessages(ctx context.Context)
}

// app owns what initApp assembled: the workers, the HTTP server and the
// shared connections. It only starts and stops them and is never handed to
// a service (ADR-059 §4).
type app struct {
	bot       updatePoller
	scheduler jobScheduler
	refresher miniAppRefresher
	server    *http.Server
	closers   []io.Closer // closed in order by Close
}

// initApp connects everything once, in this order: DB·Redis → repositories
// and external clients → Tier1 services → Telegram flows, scheduler and Mini
// App handler → HTTP router. A failure after the connections are open closes
// them before returning.
func initApp(cfg *config.Config) (_ *app, err error) {
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

	// Content collection has no scheduled job (ADR-057); Orchestrator stays nil.
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
	a.scheduler = scheduler.New(schedDeps)

	// The Mini App only reports a graded handwriting answer; SessionFlow
	// refreshes the Telegram message.
	miniappHandler := miniapp.NewHandler(miniapp.HandlerDeps{
		Session: svc.session,
		Tip:     svc.tip,
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
// once every worker has stopped.
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
// the scheduler. Connections stay open until Close.
func (a *app) shutdown() {
	log.Println("Shutting down...")

	ctx, cancel := context.WithTimeout(
		context.Background(),
		shutdownTimeout,
	)
	defer cancel()

	a.bot.Stop()
	if err := a.server.Shutdown(ctx); err != nil {
		log.Printf(
			"HTTP server shutdown error: %v",
			err,
		)
	}
	log.Println("Server stopped")

	a.scheduler.Stop()
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
