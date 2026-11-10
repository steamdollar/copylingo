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
)

// OpenDB connects to PostgreSQL. Pool tuning stays with the caller because
// only the long-running server needs it.
func OpenDB(dbConfig config.DBConfig) (*sqlx.DB, error) {
	db, err := sqlx.Connect(
		"postgres",
		dbConfig.DSN(),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"connect database: %w",
			err,
		)
	}
	return db, nil
}

// NewAudioStore builds the S3-compatible object store that holds listening
// audio. The server reads clips from it; the admin tool writes them (ADR-082).
func NewAudioStore(storageConfig config.StorageConfig) *external.S3AudioStore {
	return external.NewS3AudioStore(external.S3Options{
		Endpoint:     storageConfig.Endpoint,
		Region:       storageConfig.Region,
		Bucket:       storageConfig.Bucket,
		AccessKey:    storageConfig.AccessKey,
		SecretKey:    storageConfig.SecretKey,
		UsePathStyle: storageConfig.UsePathStyle,
	})
}
