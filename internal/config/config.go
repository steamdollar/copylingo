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
	Mode          string `mapstructure:"mode"`            // debug, release
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

func (c DBConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode,
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
	Model         string `mapstructure:"model"`            // Chat model.
	BaseURL       string `mapstructure:"base_url"`         // OpenAI-compatible endpoint.
	TTSModel      string `mapstructure:"tts_model"`        // Gemini native TTS model (ADR-031).
	TTSVoiceName  string `mapstructure:"tts_voice_name"`   // Voice A; also used for single-speaker audio.
	TTSVoiceNameB string `mapstructure:"tts_voice_name_b"` // Voice B for dialogue audio.
}

// StorageConfig configures the S3-compatible object store that holds TTS audio
// (ADR-032). "S3" here is the API standard, not the AWS vendor: only Endpoint
// changes between local MinIO and prod AWS S3. Endpoint empty => default AWS
// endpoint for Region.
type StorageConfig struct {
	Endpoint     string `mapstructure:"endpoint"`       // e.g. http://localhost:9000 (MinIO); empty for AWS S3.
	Region       string `mapstructure:"region"`         // e.g. ap-northeast-2 (Seoul).
	Bucket       string `mapstructure:"bucket"`         // e.g. copylingo-audio.
	AccessKey    string `mapstructure:"access_key"`     // static credentials; env-injected in prod.
	SecretKey    string `mapstructure:"secret_key"`     // static credentials; env-injected in prod.
	UsePathStyle bool   `mapstructure:"use_path_style"` // true for MinIO (path-style addressing); false for AWS S3 virtual-hosted style.
}

type LoggingConfig struct {
	Dir           string `mapstructure:"dir"`
	Level         string `mapstructure:"level"`
	RetentionDays int    `mapstructure:"retention_days"`
	Timezone      string `mapstructure:"timezone"`
}

func (c *Config) validate() error {
	if c.Telegram.Token == "" {
		return fmt.Errorf("telegram.token is required")
	}
	if c.LLM.APIKey == "" {
		log.Println("[WARN] llm.api_key is not set. AI features may be disabled.")
	}
	if strings.TrimSpace(c.Logging.Dir) == "" {
		return fmt.Errorf("logging.dir is required")
	}
	switch strings.ToUpper(strings.TrimSpace(c.Logging.Level)) {
	case "DEBUG", "INFO", "WARN", "ERROR":
		log.Println("log level: " + c.Logging.Level)
	default:
		return fmt.Errorf("logging.level must be one of DEBUG, INFO, WARN, ERROR")
	}
	if c.Logging.RetentionDays < 1 {
		return fmt.Errorf("logging.retention_days must be positive")
	}
	if _, err := time.LoadLocation(c.Logging.Timezone); err != nil {
		return fmt.Errorf("logging.timezone is invalid: %w", err)
	}
	return nil
}
