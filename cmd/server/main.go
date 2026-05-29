package main

import (
	"fmt"
	"log"

	_ "github.com/lib/pq"

	"github.com/lsj/copylingo/internal/config"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("Application terminated with error: %v", err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	db, rdb, cleanup, err := initInfra(cfg)
	if err != nil {
		return fmt.Errorf("failed to init infrastructure: %w", err)
	}
	defer cleanup()

	repos, services, botHandler, err := initApp(cfg, db, rdb)
	if err != nil {
		return fmt.Errorf("failed to init app: %w", err)
	}

	stopWorkers := startWorkers(cfg, services, botHandler, repos)
	defer stopWorkers()

	router := setupRouter(cfg, db, rdb, services, botHandler)
	srv := startHTTPServer(cfg, router)
	waitForShutdown(srv, botHandler)

	return nil
}
