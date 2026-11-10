package config

import (
	"fmt"
	"log"
	"strings"
	"time"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	DB       DBConfig       `mapstructure:"db"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Telegram TelegramConfig `mapstructure:"telegram"`
	LLM      LLMConfig      `mapstructure:"llm"`
	Storage  StorageConfig  `mapstructure:"storage"`
	Logging  LoggingConfig  `mapstructure:"logging"`
}

type ServerConfig struct {
	Port          int    `mapstructure:"port"`
	PublicBaseURL string `mapstructure:"public_base_url"` // HTTPS URL used by Telegram Mini Apps
}

type DBConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
}

func (config DBConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		config.Host,
		config.Port,
		config.User,
		config.Password,
		config.DBName,
		config.SSLMode,
	)
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type TelegramConfig struct {
	Token string `mapstructure:"token"`
	Debug bool   `mapstructure:"debug"`
}

// Chat uses the OpenAI-compatible endpoint; TTS uses Gemini's native endpoint.
type LLMConfig struct {
	APIKey        string `mapstructure:"api_key"`
	Model         string `mapstructure:"model"`
	BaseURL       string `mapstructure:"base_url"`
	TTSModel      string `mapstructure:"tts_model"`
	TTSVoiceName  string `mapstructure:"tts_voice_name"`   // Voice A; also used for single-speaker audio.
	TTSVoiceNameB string `mapstructure:"tts_voice_name_b"` // Voice B for dialogue audio.
}

// StorageConfig is the S3-compatible store for TTS audio; only Endpoint differs between MinIO and AWS.
type StorageConfig struct {
	Endpoint     string `mapstructure:"endpoint"` // e.g. http://localhost:9000 (MinIO); empty for AWS S3.
	Region       string `mapstructure:"region"`
	Bucket       string `mapstructure:"bucket"`
	AccessKey    string `mapstructure:"access_key"`
	SecretKey    string `mapstructure:"secret_key"`
	UsePathStyle bool   `mapstructure:"use_path_style"` // true for MinIO; false for AWS virtual-hosted style.
}

type LoggingConfig struct {
	Directory     string `mapstructure:"dir"`
	Level         string `mapstructure:"level"`
	RetentionDays int    `mapstructure:"retention_days"`
	Timezone      string `mapstructure:"timezone"`
}

// validate checks value formats only; checkRequired already rejected empty required keys.
func (config *Config) validate() error {
	if strings.TrimSpace(config.Logging.Directory) == "" {
		return fmt.Errorf("logging.dir is required")
	}
	switch strings.ToUpper(strings.TrimSpace(config.Logging.Level)) {
	case "DEBUG", "INFO", "WARN", "ERROR":
		log.Println("log level: " + config.Logging.Level)
	default:
		return fmt.Errorf("logging.level must be one of DEBUG, INFO, WARN, ERROR")
	}
	if config.Logging.RetentionDays < 1 {
		return fmt.Errorf("logging.retention_days must be positive")
	}
	if _, err := time.LoadLocation(config.Logging.Timezone); err != nil {
		return fmt.Errorf(
			"logging.timezone is invalid: %w",
			err,
		)
	}
	return nil
}
