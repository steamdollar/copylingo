package config

import (
	"os"
	"strings"
	"testing"
)

func TestLoadPrefersDotEnvPublicBaseURLOverInheritedEnvironment(t *testing.T) {
	t.Setenv(
		"COPYLINGO_TELEGRAM_TOKEN",
		"test-token",
	)
	t.Setenv(
		"COPYLINGO_LLM_API_KEY",
		"test-key",
	)
	t.Setenv(
		"COPYLINGO_SERVER_PUBLIC_BASE_URL",
		"https://old.trycloudflare.com",
	)

	temporaryDirectory := t.TempDir()
	t.Chdir(temporaryDirectory)

	if err := os.WriteFile(
		".env",
		[]byte("COPYLINGO_SERVER_PUBLIC_BASE_URL=https://fresh.trycloudflare.com\n"),
		0o600,
	); err != nil {
		t.Fatalf(
			"write .env: %v",
			err,
		)
	}

	appConfig, err := Load()
	if err != nil {
		t.Fatalf(
			"Load() error = %v",
			err,
		)
	}

	if got, want := appConfig.Server.PublicBaseURL, "https://fresh.trycloudflare.com"; got != want {
		t.Fatalf(
			"PublicBaseURL = %q, want %q",
			got,
			want,
		)
	}
}

func TestLoadUsesEnvironmentPublicBaseURLWhenDotEnvMissing(t *testing.T) {
	t.Setenv(
		"COPYLINGO_TELEGRAM_TOKEN",
		"test-token",
	)
	t.Setenv(
		"COPYLINGO_LLM_API_KEY",
		"test-key",
	)
	t.Setenv(
		"COPYLINGO_SERVER_PUBLIC_BASE_URL",
		"https://env.trycloudflare.com",
	)

	t.Chdir(t.TempDir())

	appConfig, err := Load()
	if err != nil {
		t.Fatalf(
			"Load() error = %v",
			err,
		)
	}

	if got, want := appConfig.Server.PublicBaseURL, "https://env.trycloudflare.com"; got != want {
		t.Fatalf(
			"PublicBaseURL = %q, want %q",
			got,
			want,
		)
	}
}

func TestLoadLoggingDefaults(t *testing.T) {
	t.Setenv(
		"COPYLINGO_TELEGRAM_TOKEN",
		"test-token",
	)
	t.Setenv(
		"COPYLINGO_LLM_API_KEY",
		"test-key",
	)
	t.Chdir(t.TempDir())

	appConfig, err := Load()
	if err != nil {
		t.Fatalf(
			"Load() error = %v",
			err,
		)
	}
	if got, want := appConfig.Logging.Directory, "./logs"; got != want {
		t.Fatalf(
			"Logging.Dir = %q, want %q",
			got,
			want,
		)
	}
	if got, want := appConfig.Logging.Level, "INFO"; got != want {
		t.Fatalf(
			"Logging.Level = %q, want %q",
			got,
			want,
		)
	}
	if got, want := appConfig.Logging.RetentionDays, 30; got != want {
		t.Fatalf(
			"Logging.RetentionDays = %d, want %d",
			got,
			want,
		)
	}
	if got, want := appConfig.Logging.Timezone, "Asia/Seoul"; got != want {
		t.Fatalf(
			"Logging.Timezone = %q, want %q",
			got,
			want,
		)
	}
}

func TestLoadLLMDefaults(t *testing.T) {
	t.Setenv(
		"COPYLINGO_TELEGRAM_TOKEN",
		"test-token",
	)
	t.Setenv(
		"COPYLINGO_LLM_API_KEY",
		"test-key",
	)
	t.Chdir(t.TempDir())

	appConfig, err := Load()
	if err != nil {
		t.Fatalf(
			"Load() error = %v",
			err,
		)
	}
	if got, want := appConfig.LLM.Model, "gemini-3.5-flash-lite"; got != want {
		t.Fatalf(
			"LLM.Model = %q, want %q",
			got,
			want,
		)
	}
	if got, want := appConfig.LLM.TTSModel, "gemini-2.5-flash-preview-tts"; got != want {
		t.Fatalf(
			"LLM.TTSModel = %q, want %q",
			got,
			want,
		)
	}
	if got, want := appConfig.LLM.TTSVoiceName, "Kore"; got != want {
		t.Fatalf(
			"LLM.TTSVoiceName = %q, want %q",
			got,
			want,
		)
	}
	if got, want := appConfig.LLM.TTSVoiceNameB, "Puck"; got != want {
		t.Fatalf(
			"LLM.TTSVoiceNameB = %q, want %q",
			got,
			want,
		)
	}
}

func TestLoadLLMTTSEnvironmentOverrides(t *testing.T) {
	t.Setenv(
		"COPYLINGO_TELEGRAM_TOKEN",
		"test-token",
	)
	t.Setenv(
		"COPYLINGO_LLM_API_KEY",
		"test-key",
	)
	t.Setenv(
		"COPYLINGO_LLM_TTS_MODEL",
		"test-tts-model",
	)
	t.Setenv(
		"COPYLINGO_LLM_TTS_VOICE_NAME",
		"Puck",
	)
	t.Setenv(
		"COPYLINGO_LLM_TTS_VOICE_NAME_B",
		"Zephyr",
	)
	t.Chdir(t.TempDir())

	appConfig, err := Load()
	if err != nil {
		t.Fatalf(
			"Load() error = %v",
			err,
		)
	}
	if got, want := appConfig.LLM.TTSModel, "test-tts-model"; got != want {
		t.Fatalf(
			"LLM.TTSModel = %q, want %q",
			got,
			want,
		)
	}
	if got, want := appConfig.LLM.TTSVoiceName, "Puck"; got != want {
		t.Fatalf(
			"LLM.TTSVoiceName = %q, want %q",
			got,
			want,
		)
	}
	if got, want := appConfig.LLM.TTSVoiceNameB, "Zephyr"; got != want {
		t.Fatalf(
			"LLM.TTSVoiceNameB = %q, want %q",
			got,
			want,
		)
	}
}

func TestLoadLoggingEnvironmentOverrides(t *testing.T) {
	t.Setenv(
		"COPYLINGO_TELEGRAM_TOKEN",
		"test-token",
	)
	t.Setenv(
		"COPYLINGO_LLM_API_KEY",
		"test-key",
	)
	t.Setenv(
		"COPYLINGO_LOGGING_DIR",
		"./custom-logs",
	)
	t.Setenv(
		"COPYLINGO_LOGGING_LEVEL",
		"DEBUG",
	)
	t.Setenv(
		"COPYLINGO_LOGGING_RETENTION_DAYS",
		"14",
	)
	t.Setenv(
		"COPYLINGO_LOGGING_TIMEZONE",
		"UTC",
	)
	t.Chdir(t.TempDir())

	appConfig, err := Load()
	if err != nil {
		t.Fatalf(
			"Load() error = %v",
			err,
		)
	}
	if got, want := appConfig.Logging.Directory, "./custom-logs"; got != want {
		t.Fatalf(
			"Logging.Dir = %q, want %q",
			got,
			want,
		)
	}
	if got, want := appConfig.Logging.Level, "DEBUG"; got != want {
		t.Fatalf(
			"Logging.Level = %q, want %q",
			got,
			want,
		)
	}
	if got, want := appConfig.Logging.RetentionDays, 14; got != want {
		t.Fatalf(
			"Logging.RetentionDays = %d, want %d",
			got,
			want,
		)
	}
	if got, want := appConfig.Logging.Timezone, "UTC"; got != want {
		t.Fatalf(
			"Logging.Timezone = %q, want %q",
			got,
			want,
		)
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
		t.Run(
			tt.name,
			func(t *testing.T) {
				t.Setenv(
					"COPYLINGO_TELEGRAM_TOKEN",
					"test-token",
				)
				t.Setenv(
					"COPYLINGO_LLM_API_KEY",
					"test-key",
				)
				t.Setenv(
					tt.key,
					tt.value,
				)
				t.Chdir(t.TempDir())

				if _, err := Load(); err == nil {
					t.Fatal("Load() error = nil, want validation error")
				}
			},
		)
	}
}

func TestLoadRequiresLLMAPIKey(t *testing.T) {
	t.Setenv(
		"COPYLINGO_TELEGRAM_TOKEN",
		"test-token",
	)
	t.Setenv(
		"COPYLINGO_LLM_API_KEY",
		"",
	)
	t.Chdir(t.TempDir())

	if _, err := Load(); err == nil || !strings.Contains(
		err.Error(),
		"llm.api_key is required",
	) {
		t.Fatalf(
			"Load() error = %v, want llm.api_key is required",
			err,
		)
	}
}
