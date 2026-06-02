package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/repository"
)

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
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := initDB(cfg)
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	defer db.Close()

	materials := buildVocabularyMaterials(n5Words)
	if err := repository.NewMaterialRepository(db).UpsertBatch(context.Background(), materials); err != nil {
		log.Fatalf("Failed to upsert vocabulary materials batch: %v", err)
	}

	log.Printf("Successfully upserted %d vocabulary materials.", len(materials))
}
