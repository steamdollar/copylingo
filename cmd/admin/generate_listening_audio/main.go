// Command generate_listening_audio fills missing listening-audio objects for one language.
// It is the only place that calls TTS (ADR-082); until it runs, new listening questions are never selected.
package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/lsj/copylingo/internal/bootstrap"
	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/external"
	"github.com/lsj/copylingo/internal/repository"
)

func waitForNextCycle(
	ctx context.Context,
	delay time.Duration,
) error {
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func main() {
	language := flag.String(
		"language",
		"ja",
		"question language",
	)
	timeout := flag.Duration(
		"timeout",
		10*time.Minute,
		"maximum duration for the complete batch",
	)
	batchDelay := flag.Duration(
		"batch-delay",
		0,
		"delay between TTS batches for provider rate limits",
	)
	flag.Parse()

	appConfig, err := config.Load()
	if err != nil {
		log.Fatalf(
			"load config: %v",
			err,
		)
	}

	db, err := bootstrap.OpenDB(appConfig.DB)
	if err != nil {
		log.Fatalf(
			"database connection failed: %v",
			err,
		)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(
		context.Background(),
		*timeout,
	)
	defer cancel()
	repositories := repository.NewRepositories(db)
	pending, err := repositories.Question.GetListeningNeedingAudio(
		ctx,
		*language,
		1000,
	)
	if err != nil {
		log.Fatalf(
			"load pending listening audio: %v",
			err,
		)
	}
	if len(pending) == 0 {
		log.Printf(
			"listening audio already complete language=%s",
			*language,
		)
		return
	}

	audio := newAudioGenerator(
		repositories.Question,
		external.NewTTSClient(external.TTSOptions{
			APIKey:  appConfig.LLM.APIKey,
			BaseURL: appConfig.LLM.BaseURL,
			Model:   appConfig.LLM.TTSModel,
			Voice:   appConfig.LLM.TTSVoiceName,
			VoiceB:  appConfig.LLM.TTSVoiceNameB,
		}),
		bootstrap.NewAudioStore(appConfig.Storage),
		appConfig.LLM.TTSVoiceName,
		appConfig.LLM.TTSVoiceNameB,
	)
	cycles := (len(pending) + audioGeneratePerCycle - 1) / audioGeneratePerCycle
	log.Printf(
		"filling listening audio language=%s pending=%d cycles=%d",
		*language,
		len(pending),
		cycles,
	)
	for cycle := 1; cycle <= cycles; cycle++ {
		if err := audio.TopUpAudio(
			ctx,
			*language,
		); err != nil {
			log.Fatalf(
				"generate listening audio cycle=%d/%d: %v",
				cycle,
				cycles,
				err,
			)
		}
		if cycle < cycles {
			if err := waitForNextCycle(
				ctx,
				*batchDelay,
			); err != nil {
				log.Fatalf(
					"wait between listening audio cycles=%d/%d: %v",
					cycle,
					cycles,
					err,
				)
			}
		}
	}

	remaining, err := repositories.Question.GetListeningNeedingAudio(
		ctx,
		*language,
		1000,
	)
	if err != nil {
		log.Fatalf(
			"verify listening audio: %v",
			err,
		)
	}
	if len(remaining) > 0 {
		log.Fatalf(
			"listening audio incomplete language=%s remaining=%d",
			*language,
			len(remaining),
		)
	}
	log.Printf(
		"listening audio complete language=%s generated_or_reused=%d",
		*language,
		len(pending),
	)
}
