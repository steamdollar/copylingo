package config

import (
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func setRequiredEnvironment(t *testing.T) {
	t.Helper()
	for _, key := range requiredKeys {
		t.Setenv(
			environmentVariableName(key),
			"test-"+key,
		)
	}
}

func TestLoadPrefersDotEnvOverInheritedEnvironment(t *testing.T) {
	setRequiredEnvironment(t)
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
	setRequiredEnvironment(t)
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
	setRequiredEnvironment(t)
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
	setRequiredEnvironment(t)
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
	setRequiredEnvironment(t)
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
	setRequiredEnvironment(t)
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
				setRequiredEnvironment(t)
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

func TestLoadRejectsMissingRequiredKeys(t *testing.T) {
	setRequiredEnvironment(t)
	t.Setenv(
		"COPYLINGO_DB_HOST",
		"",
	)
	t.Setenv(
		"COPYLINGO_LLM_API_KEY",
		" ",
	)
	t.Chdir(t.TempDir())

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want missing required config")
	}
	for _, name := range []string{"COPYLINGO_DB_HOST", "COPYLINGO_LLM_API_KEY"} {
		if !strings.Contains(
			err.Error(),
			name,
		) {
			t.Errorf(
				"Load() error = %v, want it to name %s",
				err,
				name,
			)
		}
	}
}

// TestEveryConfigFieldIsListedOnce: a Config field missing from the key lists would never be read from env.
func TestEveryConfigFieldIsListedOnce(t *testing.T) {
	listedCount := map[string]int{}
	for key := range defaults {
		listedCount[key]++
	}
	for _, key := range slices.Concat(
		requiredKeys,
		optionalKeys,
	) {
		listedCount[key]++
	}

	configType := reflect.TypeFor[Config]()
	for sectionIndex := range configType.NumField() {
		section := configType.Field(sectionIndex)
		for fieldIndex := range section.Type.NumField() {
			key := section.Tag.Get("mapstructure") + "." + section.Type.Field(fieldIndex).Tag.Get("mapstructure")
			if listedCount[key] != 1 {
				t.Errorf(
					"%s is listed %d times, want exactly once",
					key,
					listedCount[key],
				)
			}
			delete(
				listedCount,
				key,
			)
		}
	}
	for key := range listedCount {
		t.Errorf(
			"%s is listed but is not a Config field",
			key,
		)
	}
}
