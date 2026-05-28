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
