package repository

import (
	"context"
	"os"
	"testing"

	"github.com/jmoiron/sqlx"
)

// Regenerated audio must not be shadowed by the Telegram file_id of the old
// clip. A session-scoped temp table shadows the application's questions table.
func TestSetAudioPathClearsCachedFileIDPostgres(t *testing.T) {
	dsn := os.Getenv("COPYLINGO_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("COPYLINGO_TEST_DATABASE_URL is not set")
	}
	db, err := sqlx.Open(
		"postgres",
		dsn,
	)
	if err != nil {
		t.Fatal("open test database failed")
	}
	defer db.Close()
	// One connection keeps every statement in the session that owns the temp table.
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	for _, statement := range []string{
		`CREATE TEMP TABLE questions (id integer PRIMARY KEY, audio_path text, audio_file_id text)`,
		`INSERT INTO questions VALUES (1, NULL, 'old-clip-file-id')`,
	} {
		if _, err := db.ExecContext(
			ctx,
			statement,
		); err != nil {
			t.Fatal(err)
		}
	}

	if err := NewQuestionRepository(db).SetAudioPath(
		ctx,
		1,
		"ja/new-clip.ogg",
	); err != nil {
		t.Fatal(err)
	}

	var got struct {
		AudioPath   *string `db:"audio_path"`
		AudioFileID *string `db:"audio_file_id"`
	}
	if err := db.GetContext(
		ctx,
		&got,
		`SELECT audio_path, audio_file_id FROM questions WHERE id = 1`,
	); err != nil {
		t.Fatal(err)
	}
	if got.AudioPath == nil {
		t.Fatal("audio_path = NULL, want ja/new-clip.ogg")
	}
	if *got.AudioPath != "ja/new-clip.ogg" {
		t.Fatalf(
			"audio_path = %q, want ja/new-clip.ogg",
			*got.AudioPath,
		)
	}
	if got.AudioFileID != nil {
		t.Fatalf(
			"audio_file_id = %q, want NULL",
			*got.AudioFileID,
		)
	}
}
