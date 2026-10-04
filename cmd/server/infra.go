package main

import (
	"context"
	"log"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"github.com/lsj/copylingo/internal/config"
)

// initInfra opens the shared connections and returns their cleanup function.
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

func initDB(cfg *config.Config) (*sqlx.DB, error) {
	db, err := sqlx.Connect(
		"postgres",
		cfg.DB.DSN(),
	)
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

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	log.Println("Connected to Redis")
	return rdb, nil
}
