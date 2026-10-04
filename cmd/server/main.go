package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os/signal"
	"syscall"

	_ "github.com/lib/pq"

	"github.com/lsj/copylingo/internal/config"
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

	app, err := initApp(cfg)
	if err != nil {
		return fmt.Errorf(
			"failed to init app: %w",
			err,
		)
	}
	defer app.Close()

	// Signals are caught only once the app is assembled; until then the
	// default handler still terminates a slow startup.
	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()
	return app.Run(ctx)
}
