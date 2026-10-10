// Package bootstrap holds the setup code more than one cmd/ binary needs:
// opening connections and mapping config to external client options
// (ADR-069). Each binary still decides what it builds and how it reacts to a
// failure. Nothing under internal/ imports it.
package bootstrap

import (
	"fmt"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/external"
	"github.com/lsj/copylingo/internal/repository"
	"github.com/lsj/copylingo/internal/service"
)

// OpenDB connects to PostgreSQL. Pool tuning stays with the caller because
// only the long-running server needs it.
func OpenDB(cfg config.DBConfig) (*sqlx.DB, error) {
	db, err := sqlx.Connect(
		"postgres",
		cfg.DSN(),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"connect database: %w",
			err,
		)
	}
	return db, nil
}

// NewAudioService builds listening audio generation over the TTS settings,
// which share the LLM API key, and the object storage settings. Callers check
// cfg.LLM.APIKey first: the server runs without audio, while the admin tool
// refuses to start.
func NewAudioService(
	cfg *config.Config,
	questions *repository.QuestionRepository,
) *service.AudioService {
	return service.NewAudioService(
		questions,
		external.NewTTSClient(external.TTSOptions{
			APIKey:  cfg.LLM.APIKey,
			BaseURL: cfg.LLM.BaseURL,
			Model:   cfg.LLM.TTSModel,
			Voice:   cfg.LLM.TTSVoiceName,
			VoiceB:  cfg.LLM.TTSVoiceNameB,
		}),
		external.NewS3AudioStore(external.S3Options{
			Endpoint:     cfg.Storage.Endpoint,
			Region:       cfg.Storage.Region,
			Bucket:       cfg.Storage.Bucket,
			AccessKey:    cfg.Storage.AccessKey,
			SecretKey:    cfg.Storage.SecretKey,
			UsePathStyle: cfg.Storage.UsePathStyle,
		}),
		cfg.LLM.TTSVoiceName,
		cfg.LLM.TTSVoiceNameB,
	)
}
