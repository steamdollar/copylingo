package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/repository"
	"github.com/lsj/copylingo/internal/service"
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

	ctx := context.Background()
	repos := repository.NewRepositories(db)
	study := service.NewStudySessionService(repos.Material, repos.Session, repos.SessionMaterial)

	users, err := repos.User.GetAllUsers(ctx)
	if err != nil {
		log.Fatalf("Failed to load users: %v", err)
	}

	created := 0
	for _, user := range users {
		session, err := study.BuildStudySession(ctx, user.ID, user.Language, user.ProficiencyLevel)
		if err != nil {
			log.Fatalf("Failed to build study session user_id=%d: %v", user.ID, err)
		}
		if session == nil {
			log.Printf("No study materials available user_id=%d", user.ID)
			continue
		}
		created++
		log.Printf("Created study session user_id=%d session_id=%d materials=%d",
			user.ID, session.ID, session.TotalQuestions)
	}

	log.Printf("Created %d study sessions for %d users.", created, len(users))
}
