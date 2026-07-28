// Command generate_listening_audio fills every missing listening-audio object
// for one language/level bucket without creating or pushing learner sessions.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/external"
	"github.com/lsj/copylingo/internal/repository"
	"github.com/lsj/copylingo/internal/service"
)

func initDB(cfg *config.Config) (*sqlx.DB, error) {
	db, err := sqlx.Connect("postgres", cfg.DB.DSN())
	if err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}
	return db, nil
}

func main() {
	language := flag.String("language", "ja", "question language")
	level := flag.String("level", "N5", "proficiency level")
	timeout := flag.Duration("timeout", 10*time.Minute, "maximum duration for the complete batch")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if !cfg.TTS.Enabled || cfg.LLM.APIKey == "" {
		log.Fatal("listening TTS requires tts.enabled=true and llm.api_key")
	}

	db, err := initDB(cfg)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	repos := repository.NewRepositories(db)
	pending, err := repos.Question.GetListeningNeedingAudio(ctx, *language, *level, 1000)
	if err != nil {
		log.Fatalf("load pending listening audio: %v", err)
	}
	if len(pending) == 0 {
		log.Printf("listening audio already complete language=%s level=%s", *language, *level)
		return
	}

	audio := service.NewAudioService(
		repos.Question,
		external.NewTTSClient(cfg),
		external.NewS3AudioStore(cfg),
		cfg.TTS.VoiceName,
	)
	cycles := (len(pending) + service.AudioGeneratePerCycle - 1) / service.AudioGeneratePerCycle
	log.Printf(
		"filling listening audio language=%s level=%s pending=%d cycles=%d",
		*language,
		*level,
		len(pending),
		cycles,
	)
	for cycle := 1; cycle <= cycles; cycle++ {
		if err := audio.TopUpAudio(ctx, *language, *level); err != nil {
			log.Fatalf("generate listening audio cycle=%d/%d: %v", cycle, cycles, err)
		}
	}

	remaining, err := repos.Question.GetListeningNeedingAudio(ctx, *language, *level, 1000)
	if err != nil {
		log.Fatalf("verify listening audio: %v", err)
	}
	if len(remaining) > 0 {
		log.Fatalf("listening audio incomplete language=%s level=%s remaining=%d", *language, *level, len(remaining))
	}
	log.Printf("listening audio complete language=%s level=%s generated_or_reused=%d", *language, *level, len(pending))
}
