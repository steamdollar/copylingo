// Command reset_learning_data empties every learning table while keeping
// users, so seeds can be reloaded from scratch.
//
// Usage: go run ./cmd/admin/reset_learning_data -yes
package main

import (
	"flag"
	"log"

	"github.com/lsj/copylingo/internal/bootstrap"
	"github.com/lsj/copylingo/internal/config"
)

// users references none of these tables, so CASCADE cannot reach it.
const resetQuery = `TRUNCATE TABLE
	session_questions, session_materials, sessions,
	user_material_progress, user_material_preferences, user_question_progress,
	questions, materials, contents, tips
	RESTART IDENTITY CASCADE`

func main() {
	yes := flag.Bool(
		"yes",
		false,
		"confirm destructive reset while preserving users",
	)
	flag.Parse()
	if !*yes {
		log.Fatal("refusing to reset learning data without -yes")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf(
			"Failed to load config: %v",
			err,
		)
	}
	db, err := bootstrap.OpenDB(cfg.DB)
	if err != nil {
		log.Fatalf(
			"Database connection failed: %v",
			err,
		)
	}
	defer db.Close()

	if _, err := db.Exec(resetQuery); err != nil {
		log.Fatalf(
			"Failed to reset learning data: %v",
			err,
		)
	}
	log.Print("Reset learning data while preserving users.")
}
