package main

import (
	"context"
	"log"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"github.com/lsj/copylingo/internal/bootstrap"
	"github.com/lsj/copylingo/internal/config"
)

// initializeInfrastructure closes the DB connection if Redis fails to open.
func initializeInfrastructure(appConfig *config.Config) (*sqlx.DB, *redis.Client, error) {
	db, err := initializeDB(appConfig)
	if err != nil {
		return nil, nil, err
	}
	redisClient, err := initializeRedis(appConfig)
	if err != nil {
		db.Close()
		return nil, nil, err
	}
	return db, redisClient, nil
}

func initializeDB(appConfig *config.Config) (*sqlx.DB, error) {
	db, err := bootstrap.OpenDB(appConfig.DB)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	log.Println("Connected to PostgreSQL")
	return db, nil
}

func initializeRedis(appConfig *config.Config) (*redis.Client, error) {
	redisClient := redis.NewClient(&redis.Options{
		Addr:     appConfig.Redis.Addr,
		Password: appConfig.Redis.Password,
		DB:       appConfig.Redis.DB,
	})

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	log.Println("Connected to Redis")
	return redisClient, nil
}
