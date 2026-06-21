package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"github.com/lsj/copylingo/internal/config"
)

var learningDataTables = []string{
	"session_questions",
	"session_materials",
	"sessions",
	"user_material_progress",
	"questions",
	"materials",
	"contents",
	"tips",
}

func initDB(cfg *config.Config) (*sqlx.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.DB.Host, cfg.DB.Port, cfg.DB.User, cfg.DB.Password, cfg.DB.DBName, cfg.DB.SSLMode)
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}
	return db, nil
}

func main() {
	yes := flag.Bool("yes", false, "confirm destructive reset while preserving users")
	flag.Parse()
	if !*yes {
		log.Fatal("refusing to reset learning data without -yes")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := initDB(cfg)
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	defer db.Close()

	if err := resetLearningData(context.Background(), db); err != nil {
		log.Fatalf("Failed to reset learning data: %v", err)
	}
	log.Printf("Reset learning data tables while preserving users: %v", learningDataTables)
}

func resetLearningData(ctx context.Context, db *sqlx.DB) error {
	query := buildResetLearningDataQuery(learningDataTables)
	if _, err := db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("reset learning data: %w", err)
	}
	return nil
}

func buildResetLearningDataQuery(tables []string) string {
	return "TRUNCATE TABLE " + joinIdentifiers(tables) + " RESTART IDENTITY CASCADE"
}

func joinIdentifiers(values []string) string {
	if len(values) == 0 {
		return ""
	}

	out := values[0]
	for _, value := range values[1:] {
		out += ", " + value
	}
	return out
}
