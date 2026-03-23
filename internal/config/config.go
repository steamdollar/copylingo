package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	DB       DBConfig       `mapstructure:"db"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Telegram TelegramConfig `mapstructure:"telegram"`
	OpenAI   OpenAIConfig   `mapstructure:"openai"`
	TTS      TTSConfig      `mapstructure:"tts"`
	Schedule ScheduleConfig `mapstructure:"schedule"`
}

type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"` // debug, release
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

type OpenAIConfig struct {
	APIKey string `mapstructure:"api_key"`
	Model  string `mapstructure:"model"` // gpt-4o-mini
}

type TTSConfig struct {
	Enabled      bool   `mapstructure:"enabled"`
	CredPath     string `mapstructure:"cred_path"` // Google Cloud credentials JSON path
	AudioDir     string `mapstructure:"audio_dir"`
	LanguageCode string `mapstructure:"language_code"` // ja-JP
	VoiceName    string `mapstructure:"voice_name"`    // ja-JP-Neural2-B
}

type ScheduleConfig struct {
	ContentCollectCron string `mapstructure:"content_collect_cron"` // 콘텐츠 수집 크론
	MorningBuildCron   string `mapstructure:"morning_build_cron"`   // 오전 세션 빌드 크론
	MorningPushCron    string `mapstructure:"morning_push_cron"`    // 오전 세션 푸시 크론
	EveningBuildCron   string `mapstructure:"evening_build_cron"`   // 오후 세션 빌드 크론
	EveningPushCron    string `mapstructure:"evening_push_cron"`    // 오후 세션 푸시 크론
}

// Load reads config from file and environment variables.
func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("/etc/copylingo")

	// Environment variable overrides: COPYLINGO_DB_HOST, COPYLINGO_TELEGRAM_TOKEN, etc.
	viper.SetEnvPrefix("COPYLINGO")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Defaults
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.mode", "debug")
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
	viper.SetDefault("openai.model", "gpt-4o-mini")
	viper.SetDefault("tts.enabled", true)
	viper.SetDefault("tts.audio_dir", "./data/audio")
	viper.SetDefault("tts.language_code", "ja-JP")
	viper.SetDefault("tts.voice_name", "ja-JP-Neural2-B")
	viper.SetDefault("schedule.content_collect_cron", "0 3 * * *")  // 매일 03:00
	viper.SetDefault("schedule.morning_build_cron", "30 7 * * *")   // 매일 07:30
	viper.SetDefault("schedule.morning_push_cron", "0 8 * * *")     // 매일 08:00
	viper.SetDefault("schedule.evening_build_cron", "30 20 * * *")  // 매일 20:30
	viper.SetDefault("schedule.evening_push_cron", "0 21 * * *")    // 매일 21:00

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// Config file not found is OK — use defaults + env vars
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

func (c *Config) validate() error {
	if c.Telegram.Token == "" {
		return fmt.Errorf("telegram.token is required")
	}
	if c.OpenAI.APIKey == "" {
		return fmt.Errorf("openai.api_key is required")
	}
	return nil
}
