package config

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

const environmentPrefix = "COPYLINGO"

// Every Config key is read from COPYLINGO_<SECTION>_<KEY> and belongs to exactly one list below.

var defaults = map[string]any{
	"server.port":    8080,
	"db.port":        5432,
	"telegram.debug": true, // Kept on on purpose: Bot API traffic is logged for troubleshooting.
	"llm.model":      "gemini-3.5-flash-lite",
	// OpenAI-compatible endpoint for chat; TTS uses Gemini's native endpoint.
	"llm.base_url":           "https://generativelanguage.googleapis.com/v1beta/openai/",
	"llm.tts_model":          "gemini-2.5-flash-preview-tts", // Swap via env if the preview id changes.
	"llm.tts_voice_name":     "Kore",
	"llm.tts_voice_name_b":   "Puck",
	"logging.dir":            "./logs",
	"logging.level":          "INFO",
	"logging.retention_days": 30,
	"logging.timezone":       "Asia/Seoul",
}

// requiredKeys are secrets or differ per environment (local containers vs
// cloud), so they have no default: a missing one fails Load instead of
// silently pointing at localhost.
var requiredKeys = []string{
	"telegram.token",
	"llm.api_key",
	"db.host",
	"db.user",
	"db.password",
	"db.dbname",
	"db.sslmode",
	"redis.addr",
	"storage.region",
	"storage.bucket",
	"storage.access_key",
	"storage.secret_key",
}

// optionalKeys have no default because their zero value is a valid setting,
// e.g. an empty storage.endpoint selects the AWS S3 endpoint.
var optionalKeys = []string{
	"server.public_base_url",
	"redis.password",
	"redis.db",
	"storage.endpoint",
	"storage.use_path_style",
}

// Load reads config from environment variables. A local .env file, when
// present, overrides inherited variables so a refreshed .env (e.g. a new
// tunnel URL) wins over a stale shell export; deployed containers have no
// .env and receive every value from the platform.
func Load() (*Config, error) {
	// A missing .env is normal outside local development.
	_ = godotenv.Overload()

	settings := viper.New()
	settings.SetEnvPrefix(environmentPrefix)
	settings.SetEnvKeyReplacer(strings.NewReplacer(
		".",
		"_",
	))
	for key, value := range defaults {
		settings.SetDefault(
			key,
			value,
		)
	}
	// Bind every key so environment-only values reach Unmarshal.
	for _, key := range slices.Concat(
		slices.Collect(maps.Keys(defaults)),
		requiredKeys,
		optionalKeys,
	) {
		if err := settings.BindEnv(key); err != nil {
			return nil, fmt.Errorf(
				"bind env %s: %w",
				key,
				err,
			)
		}
	}

	if err := checkRequired(settings); err != nil {
		return nil, err
	}

	var appConfig Config
	if err := settings.Unmarshal(&appConfig); err != nil {
		return nil, fmt.Errorf(
			"failed to unmarshal config: %w",
			err,
		)
	}

	if err := appConfig.validate(); err != nil {
		return nil, fmt.Errorf(
			"config validation failed: %w",
			err,
		)
	}

	return &appConfig, nil
}

// checkRequired names every empty required key so one failed start lists everything left to set.
func checkRequired(settings *viper.Viper) error {
	var missing []string
	for _, key := range requiredKeys {
		if strings.TrimSpace(settings.GetString(key)) == "" {
			missing = append(
				missing,
				environmentVariableName(key),
			)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf(
			"missing required config: %s",
			strings.Join(
				missing,
				", ",
			),
		)
	}
	return nil
}

func environmentVariableName(key string) string {
	return environmentPrefix + "_" + strings.ToUpper(strings.ReplaceAll(
		key,
		".",
		"_",
	))
}
