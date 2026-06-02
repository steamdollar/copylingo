package config

import (
	"os"
	"testing"
)

func TestLoadPrefersDotEnvPublicBaseURLOverInheritedEnv(t *testing.T) {
	t.Setenv("COPYLINGO_TELEGRAM_TOKEN", "test-token")
	t.Setenv("COPYLINGO_SERVER_PUBLIC_BASE_URL", "https://old.trycloudflare.com")

	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	if err := os.WriteFile(".env", []byte("COPYLINGO_SERVER_PUBLIC_BASE_URL=https://fresh.trycloudflare.com\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got, want := cfg.Server.PublicBaseURL, "https://fresh.trycloudflare.com"; got != want {
		t.Fatalf("PublicBaseURL = %q, want %q", got, want)
	}
}

func TestLoadUsesEnvPublicBaseURLWhenDotEnvMissing(t *testing.T) {
	t.Setenv("COPYLINGO_TELEGRAM_TOKEN", "test-token")
	t.Setenv("COPYLINGO_SERVER_PUBLIC_BASE_URL", "https://env.trycloudflare.com")

	t.Chdir(t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got, want := cfg.Server.PublicBaseURL, "https://env.trycloudflare.com"; got != want {
		t.Fatalf("PublicBaseURL = %q, want %q", got, want)
	}
}

func TestLoadLoggingDefaults(t *testing.T) {
	t.Setenv("COPYLINGO_TELEGRAM_TOKEN", "test-token")
	t.Chdir(t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got, want := cfg.Logging.Dir, "./logs"; got != want {
		t.Fatalf("Logging.Dir = %q, want %q", got, want)
	}
	if got, want := cfg.Logging.Level, "INFO"; got != want {
		t.Fatalf("Logging.Level = %q, want %q", got, want)
	}
	if got, want := cfg.Logging.RetentionDays, 30; got != want {
		t.Fatalf("Logging.RetentionDays = %d, want %d", got, want)
	}
	if got, want := cfg.Logging.Timezone, "Asia/Seoul"; got != want {
		t.Fatalf("Logging.Timezone = %q, want %q", got, want)
	}
}

func TestLoadLoggingEnvOverrides(t *testing.T) {
	t.Setenv("COPYLINGO_TELEGRAM_TOKEN", "test-token")
	t.Setenv("COPYLINGO_LOGGING_DIR", "./custom-logs")
	t.Setenv("COPYLINGO_LOGGING_LEVEL", "DEBUG")
	t.Setenv("COPYLINGO_LOGGING_RETENTION_DAYS", "14")
	t.Setenv("COPYLINGO_LOGGING_TIMEZONE", "UTC")
	t.Chdir(t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got, want := cfg.Logging.Dir, "./custom-logs"; got != want {
		t.Fatalf("Logging.Dir = %q, want %q", got, want)
	}
	if got, want := cfg.Logging.Level, "DEBUG"; got != want {
		t.Fatalf("Logging.Level = %q, want %q", got, want)
	}
	if got, want := cfg.Logging.RetentionDays, 14; got != want {
		t.Fatalf("Logging.RetentionDays = %d, want %d", got, want)
	}
	if got, want := cfg.Logging.Timezone, "UTC"; got != want {
		t.Fatalf("Logging.Timezone = %q, want %q", got, want)
	}
}

func TestLoadRejectsInvalidLoggingConfig(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "empty directory", key: "COPYLINGO_LOGGING_DIR", value: " "},
		{name: "invalid level", key: "COPYLINGO_LOGGING_LEVEL", value: "TRACE"},
		{name: "invalid retention", key: "COPYLINGO_LOGGING_RETENTION_DAYS", value: "0"},
		{name: "invalid timezone", key: "COPYLINGO_LOGGING_TIMEZONE", value: "Not/AZone"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("COPYLINGO_TELEGRAM_TOKEN", "test-token")
			t.Setenv(tt.key, tt.value)
			t.Chdir(t.TempDir())

			if _, err := Load(); err == nil {
				t.Fatal("Load() error = nil, want validation error")
			}
		})
	}
}
