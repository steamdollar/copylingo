package config

import (
	"fmt"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// Load reads config from file and environment variables.
func Load() (*Config, error) {
	viper.Reset()

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	// Load .env file if it exists
	dotEnv, _ := godotenv.Read()
	_ = godotenv.Load()

	// Environment variable overrides: COPYLINGO_DB_HOST, COPYLINGO_TELEGRAM_TOKEN, etc.
	viper.SetEnvPrefix("COPYLINGO")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	if err := bindEnv(); err != nil {
		return nil, err
	}

	setDefaults()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// Config file not found is OK — use defaults + env vars
	}

	if publicBaseURL := strings.TrimSpace(dotEnv["COPYLINGO_SERVER_PUBLIC_BASE_URL"]); publicBaseURL != "" {
		viper.Set("server.public_base_url", publicBaseURL)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

func bindEnv() error {
	// Register every Config field so environment-only values reach Unmarshal.
	keys := []string{
		"server.port",
		"server.mode",
		"server.public_base_url",
		"db.host",
		"db.port",
		"db.user",
		"db.password",
		"db.dbname",
		"db.sslmode",
		"redis.addr",
		"redis.password",
		"redis.db",
		"telegram.token",
		"telegram.debug",
		"llm.api_key",
		"llm.model",
		"llm.base_url",
		"llm.tts_model",
		"llm.tts_voice_name",
		"llm.tts_voice_name_b",
		"storage.endpoint",
		"storage.region",
		"storage.bucket",
		"storage.access_key",
		"storage.secret_key",
		"storage.use_path_style",
		"logging.dir",
		"logging.level",
		"logging.retention_days",
		"logging.timezone",
	}
	for _, key := range keys {
		if err := viper.BindEnv(key); err != nil {
			return fmt.Errorf("bind env %s: %w", key, err)
		}
	}
	return nil
}

func setDefaults() {
	// Defaults
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.mode", "debug")
	viper.SetDefault("server.public_base_url", "")
	viper.SetDefault("db.host", "localhost")
	viper.SetDefault("db.port", 5432)
	viper.SetDefault("db.user", "copylingo")
	viper.SetDefault("db.password", "copylingo")
	viper.SetDefault("db.dbname", "copylingo")
	viper.SetDefault("db.sslmode", "disable")
	viper.SetDefault("redis.addr", "localhost:6379")
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("telegram.debug", false)

	// llm
	viper.SetDefault("llm.model", "gemini-3.5-flash-lite") // default to LLM model
	viper.SetDefault(
		"llm.base_url",
		"https://generativelanguage.googleapis.com/v1beta/openai/",
	) // LLM compatibility layer
	// Native TTS shares the LLM API key but uses its own model and voice.
	viper.SetDefault(
		"llm.tts_model",
		"gemini-2.5-flash-preview-tts",
	) // ADR-031; swap via env if the preview id changes.
	viper.SetDefault("llm.tts_voice_name", "Kore")   // Gemini prebuilt voice (ADR-031).
	viper.SetDefault("llm.tts_voice_name_b", "Puck") // Distinct voice for speaker B in dialogues.

	// storage (S3-compatible object store; local defaults target the MinIO container)
	viper.SetDefault("storage.endpoint", "http://localhost:9000")
	viper.SetDefault("storage.region", "ap-northeast-2")
	viper.SetDefault("storage.bucket", "copylingo-audio")
	viper.SetDefault("storage.access_key", "minioadmin")
	viper.SetDefault("storage.secret_key", "minioadmin")
	viper.SetDefault("storage.use_path_style", true)

	// logging
	viper.SetDefault("logging.dir", "./logs")
	viper.SetDefault("logging.level", "INFO")
	viper.SetDefault("logging.retention_days", 30)
	viper.SetDefault("logging.timezone", "Asia/Seoul")
}
