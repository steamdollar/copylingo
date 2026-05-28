package config

import (
	"fmt"
	"log"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	DB       DBConfig       `mapstructure:"db"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Telegram TelegramConfig `mapstructure:"telegram"`
	LLM      LLMConfig      `mapstructure:"llm"`
	TTS      TTSConfig      `mapstructure:"tts"`
	Schedule ScheduleConfig `mapstructure:"schedule"`
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

// LLMConfig는 시스템 전반의 LLM(거대 언어 모델) 설정을 담당
// 구조체명은 LLMConfig이며 통신에 표준 go-openai 패키지를 사용할 수 있도록 호환 계층을 둠
// 이는 Google API가 'OpenAI 호환 모드(Compatibility Layer)'를 지원하기 때문에 가능
// BaseURL을 구글 측 엔드포인트로 덮어씌우게 되면, 향후 다른 LLM(Gemini, GPT-4o, Claude 등)으로
// 마이그레이션이 필요할 때 로직 코드 수정 전혀 없이 환경변수(BaseURL, API Key)만으로 즉각 교체할 수 있어 유지보수성이 극대화
type LLMConfig struct {
	APIKey  string `mapstructure:"api_key"`
	Model   string `mapstructure:"model"`    // gpt-4o-mini
	BaseURL string `mapstructure:"base_url"` // https://generativelanguage.googleapis.com/v1beta/openai/
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
	viper.Reset()

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("/etc/copylingo")

	// Load .env file if it exists
	dotEnv, _ := godotenv.Read()
	_ = godotenv.Load()

	// Environment variable overrides: COPYLINGO_DB_HOST, COPYLINGO_TELEGRAM_TOKEN, etc.
	viper.SetEnvPrefix("COPYLINGO")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
	if err := bindEnv(); err != nil {
		return nil, err
	}

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
	viper.SetDefault("llm.model", "gemini-3.1-flash-lite")                                       // default to LLM model
	viper.SetDefault("llm.base_url", "https://generativelanguage.googleapis.com/v1beta/openai/") // LLM compatibility layer
	viper.SetDefault("tts.enabled", true)
	viper.SetDefault("tts.audio_dir", "./data/audio")
	viper.SetDefault("tts.language_code", "ja-JP")
	viper.SetDefault("tts.voice_name", "ja-JP-Neural2-B")
	viper.SetDefault("schedule.content_collect_cron", "0 3 * * *") // 매일 03:00
	viper.SetDefault("schedule.morning_build_cron", "30 7 * * *")  // 매일 07:30
	viper.SetDefault("schedule.morning_push_cron", "0 8 * * *")    // 매일 08:00
	viper.SetDefault("schedule.evening_build_cron", "30 20 * * *") // 매일 20:30
	viper.SetDefault("schedule.evening_push_cron", "0 21 * * *")   // 매일 21:00

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
		"tts.enabled",
		"tts.cred_path",
		"tts.audio_dir",
		"tts.language_code",
		"tts.voice_name",
		"schedule.content_collect_cron",
		"schedule.morning_build_cron",
		"schedule.morning_push_cron",
		"schedule.evening_build_cron",
		"schedule.evening_push_cron",
	}
	for _, key := range keys {
		if err := viper.BindEnv(key); err != nil {
			return fmt.Errorf("bind env %s: %w", key, err)
		}
	}
	return nil
}

func (c *Config) validate() error {
	if c.Telegram.Token == "" {
		return fmt.Errorf("telegram.token is required")
	}
	if c.LLM.APIKey == "" {
		log.Println("[WARN] llm.api_key is not set. AI features may be disabled.")
	}
	return nil
}
