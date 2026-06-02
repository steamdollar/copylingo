package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContextHandlerInjectsCorrelationAttributes(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	logger := slog.New(NewContextHandler(slog.NewJSONHandler(&output, nil)))
	ctx := WithAttrs(context.Background(),
		slog.String("interaction_id", "http-test"),
		slog.String("source", "http"),
		slog.Int64("user_id", 123),
	)

	logger.InfoContext(ctx, "completed", "event", "http.completed", "status", 200)

	entry := decodeLogEntry(t, output.Bytes())
	for key, want := range map[string]any{
		"interaction_id": "http-test",
		"source":         "http",
		"event":          "http.completed",
		"user_id":        float64(123),
		"status":         float64(200),
	} {
		if got := entry[key]; got != want {
			t.Fatalf("entry[%q] = %#v, want %#v", key, got, want)
		}
	}
}

func TestNewLoggerRoutesLegacyLogPackageToJSON(t *testing.T) {
	dir := t.TempDir()
	var stdout bytes.Buffer
	logger, closeLogger, err := NewLogger(LoggerOptions{
		Dir:           dir,
		Level:         "INFO",
		RetentionDays: 30,
		Timezone:      "Asia/Seoul",
		Stdout:        &stdout,
	})
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer closeLogger()

	previous := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(previous)

	log.Print("legacy message")

	entry := decodeLogEntry(t, stdout.Bytes())
	if got := entry["msg"]; got != "legacy message" {
		t.Fatalf("entry[msg] = %#v, want legacy message", got)
	}
	if got := entry["event"]; got != "legacy.log" {
		t.Fatalf("entry[event] = %#v, want legacy.log", got)
	}
	if got := entry["source"]; got != "app" {
		t.Fatalf("entry[source] = %#v, want app", got)
	}
	if got := entry["time"].(string); !strings.Contains(got, "+09:00") {
		t.Fatalf("entry[time] = %q, want Asia/Seoul offset", got)
	}
}

func TestNewLoggerFallsBackToStdoutWhenFileSinkFails(t *testing.T) {
	dir := t.TempDir()
	notDirectory := filepath.Join(dir, "file")
	if err := os.WriteFile(notDirectory, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	logger, closeLogger, err := NewLogger(LoggerOptions{
		Dir:           notDirectory,
		Level:         "INFO",
		RetentionDays: 30,
		Timezone:      "Asia/Seoul",
		Stdout:        &stdout,
		Stderr:        &stderr,
	})
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer closeLogger()

	logger.Info("stdout remains available", "event", "test.completed")

	if !strings.Contains(stdout.String(), `"event":"test.completed"`) {
		t.Fatalf("stdout = %q, want structured fallback log", stdout.String())
	}
	if !strings.Contains(stderr.String(), "file sink unavailable; using stdout only") {
		t.Fatalf("stderr = %q, want fallback warning", stderr.String())
	}
}

func decodeLogEntry(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var entry map[string]any
	if err := json.Unmarshal(body, &entry); err != nil {
		t.Fatalf("json.Unmarshal(%q) error = %v", body, err)
	}
	return entry
}
