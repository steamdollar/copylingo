// Package bootstrap holds setup shared by more than one cmd/ binary (ADR-069); nothing under internal/ imports it.
package bootstrap

import (
	"fmt"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/external"
)

// OpenDB leaves pool tuning to the caller; only the long-running server needs it.
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

// NewAudioStore builds the listening-audio store: the server reads clips, the admin tool writes them (ADR-082).
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
