package main

import (
	"fmt"
	"log"
	"log/slog"

	_ "github.com/lib/pq"

	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/miniapp"
	"github.com/lsj/copylingo/internal/observability"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf(
			"Application terminated with error: %v",
			err,
		)
	}
}

func run() error {
	// load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf(
			"failed to load config: %w",
			err,
		)
	}

	// set logger
	logger, closeLogger, err := observability.NewLogger(observability.LoggerOptions{
		Dir:           cfg.Logging.Dir,
		Level:         cfg.Logging.Level,
		RetentionDays: cfg.Logging.RetentionDays,
		Timezone:      cfg.Logging.Timezone,
	})
	if err != nil {
		return fmt.Errorf(
			"failed to initialize logging: %w",
			err,
		)
	}
	defer closeLogger()
	slog.SetDefault(logger)

	// db, redis set up
	db, rdb, cleanup, err := initInfra(cfg)
	if err != nil {
		return fmt.Errorf(
			"failed to init infrastructure: %w",
			err,
		)
	}
	defer cleanup()

	// initialize application components
	svc, components, err := initApp(
		cfg,
		db,
		rdb,
	)
	if err != nil {
		return fmt.Errorf(
			"failed to init app: %w",
			err,
		)
	}

	stopWorkers := startWorkers(
		svc,
		components,
		rdb,
	)
	defer stopWorkers()

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
	router := setupRouter(
		cfg,
		db,
		rdb,
		miniappHandler,
	)
	srv := startHTTPServer(
		cfg,
		router,
	)

	// wait for shutdown
	waitForShutdown(
		srv,
		components.router,
	)

	return nil
}
