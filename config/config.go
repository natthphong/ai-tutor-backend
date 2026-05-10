package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Env              string
	Server           Server
	LogConfig        LogConfig
	DBConfig         DBConfig
	HTTP             HTTP
	PermissionConfig map[string][]string
	Gemini           GeminiConfig
	OpenRouter       OpenRouterConfig
	OpenAI           OpenAIConfig
	LLM              LLMConfig
	TTS              TTSConfig
	STT              STTConfig
	Embedding        EmbeddingConfig
	MinIO            MinIOConfig
	Redis            RedisConfig
	Queue            QueueConfig
	Cache            CacheConfig
	Line             LineConfig
	Tavily           TavilyConfig
	Tutor            TutorConfig
}

type Server struct {
	Name string
	Port string
}

type LogConfig struct {
	Level string
}

type DBConfig struct {
	Host            string
	Port            string
	Username        string
	Password        string
	Name            string
	MaxOpenConn     int32
	MaxConnLifeTime int64
}

type HTTP struct {
	TimeOut            time.Duration
	MaxIdleConn        int
	MaxIdleConnPerHost int
	MaxConnPerHost     int
}

type GeminiConfig struct {
	APIKey string `mapstructure:"api_key"`
}

type OpenRouterConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
}

type OpenAIConfig struct {
	APIKey   string `mapstructure:"api_key"`
	STTModel string `mapstructure:"stt_model"`
	TTSModel string `mapstructure:"tts_model"`
}

type LLMConfig struct {
	PrimaryProvider  string   `mapstructure:"primary_provider"`
	PrimaryModel     string   `mapstructure:"primary_model"`
	DeepModel        string   `mapstructure:"deep_model"`
	FallbackProvider string   `mapstructure:"fallback_provider"`
	FallbackModels   []string `mapstructure:"fallback_models"`
	TimeoutSeconds   int      `mapstructure:"timeout_seconds"`
}

type TTSConfig struct {
	PrimaryProvider  string `mapstructure:"primary_provider"`
	FallbackProvider string `mapstructure:"fallback_provider"`
	FallbackModel    string `mapstructure:"fallback_model"`
	Enabled          bool
	MaxTextChars     int    `mapstructure:"max_text_chars"`
	VoiceStyle       string `mapstructure:"voice_style"`
}

type STTConfig struct {
	PrimaryProvider  string `mapstructure:"primary_provider"`
	FallbackProvider string `mapstructure:"fallback_provider"`
	FallbackModel    string `mapstructure:"fallback_model"`
	SaveTranscript   bool   `mapstructure:"save_transcript"`
}

type EmbeddingConfig struct {
	Provider          string `mapstructure:"provider"`
	Model             string `mapstructure:"model"`
	Dimensions        int    `mapstructure:"dimensions"`
	BatchSize         int    `mapstructure:"batch_size"`
	CacheByContentHash bool  `mapstructure:"cache_by_content_hash"`
}

type MinIOConfig struct {
	Endpoint        string `mapstructure:"endpoint"`
	AccessKey       string `mapstructure:"access_key"`
	SecretKey       string `mapstructure:"secret_key"`
	Bucket          string `mapstructure:"bucket"`
	UseSSL          bool   `mapstructure:"use_ssl"`
	PrefixTTS       string `mapstructure:"prefix_tts"`
	PrefixUserAudio string `mapstructure:"prefix_user_audio"`
	PublicBase      string `mapstructure:"public_base"`
}

type RedisConfig struct {
	Enabled  bool
	Mode     string
	Host     string
	Port     string
	Addr     string
	Password string
	DB       int
	Cluster  RedisClusterConfig
}

type RedisClusterConfig struct {
	Addr []string
}

type KafkaConfig struct {
	Brokers  []string
	Group    string
	Version  string
	Oldest   bool
	SSAL     bool
	TLS      bool
	Username string
	Password string
	Certs    string
	Strategy string
	Topics   []string
}

type QueueConfig struct {
	Driver string
}

type CacheConfig struct {
	Driver     string
	TTLSeconds int `mapstructure:"ttl_seconds"`
}

type LineConfig struct {
	ChannelSecret  string   `mapstructure:"channel_secret"`
	ChannelToken   string   `mapstructure:"channel_token"`
	LiffID         string   `mapstructure:"liff_id"`
	AllowedUserIDs []string `mapstructure:"allowed_user_ids"`
	NotifyURL      string   `mapstructure:"notify_url"`
}

type TavilyConfig struct {
	APIKey string `mapstructure:"api_key"`
}

type TutorConfig struct {
	LecturePath        string `mapstructure:"lecture_path"`
	MaxHintLevel       int    `mapstructure:"max_hint_level"`
	WeaknessThreshold  int    `mapstructure:"weakness_threshold"`
	DefaultMode        string `mapstructure:"default_mode"`
}

func InitConfig() (*Config, error) {
	viper.SetDefault("LogConfig.LEVEL", "info")
	viper.SetDefault("Server.Name", "ai-tutor")
	viper.SetDefault("Server.Port", "8080")

	// LLM defaults
	viper.SetDefault("LLM.primary_provider", "gemini")
	viper.SetDefault("LLM.primary_model", "gemini-2.5-flash")
	viper.SetDefault("LLM.deep_model", "gemini-2.5-pro")
	viper.SetDefault("LLM.fallback_provider", "openrouter")
	viper.SetDefault("LLM.timeout_seconds", 60)

	// TTS defaults
	viper.SetDefault("TTS.primary_provider", "gemini")
	viper.SetDefault("TTS.fallback_provider", "openai")
	viper.SetDefault("TTS.fallback_model", "gpt-4o-mini-tts")
	viper.SetDefault("TTS.enabled", true)
	viper.SetDefault("TTS.max_text_chars", 700)
	viper.SetDefault("TTS.voice_style", "friendly Thai-English teacher")

	// STT defaults
	viper.SetDefault("STT.primary_provider", "gemini")
	viper.SetDefault("STT.fallback_provider", "openai")
	viper.SetDefault("STT.fallback_model", "whisper-1")

	// Embedding defaults
	viper.SetDefault("Embedding.provider", "openai")
	viper.SetDefault("Embedding.model", "text-embedding-3-small")
	viper.SetDefault("Embedding.dimensions", 1536)
	viper.SetDefault("Embedding.batch_size", 32)

	// MinIO defaults
	viper.SetDefault("MinIO.endpoint", "localhost:9000")
	viper.SetDefault("MinIO.access_key", "minioadmin")
	viper.SetDefault("MinIO.secret_key", "minioadmin")
	viper.SetDefault("MinIO.bucket", "ai-tutor")
	viper.SetDefault("MinIO.prefix_tts", "tts/")
	viper.SetDefault("MinIO.prefix_user_audio", "user-audio/")

	// Queue/Cache defaults
	viper.SetDefault("Queue.driver", "memory_channel")
	viper.SetDefault("Cache.driver", "memory")
	viper.SetDefault("Cache.ttl_seconds", 3600)

	// Tutor defaults
	viper.SetDefault("Tutor.lecture_path", "../lecture")
	viper.SetDefault("Tutor.max_hint_level", 5)
	viper.SetDefault("Tutor.weakness_threshold", 5)
	viper.SetDefault("Tutor.default_mode", "mixed")

	// OpenRouter default
	viper.SetDefault("OpenRouter.base_url", "https://openrouter.ai/api/v1")

	configPath, ok := os.LookupEnv("API_CONFIG_PATH")
	if !ok {
		configPath = "./config"
	}
	configName, ok := os.LookupEnv("API_CONFIG_NAME")
	if !ok {
		configName = "config"
	}

	viper.SetConfigName(configName)
	viper.AddConfigPath(configPath)

	if err := viper.ReadInConfig(); err != nil {
		fmt.Println("config file not found. using default/env config: " + err.Error())
	}

	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	var c Config
	err := viper.Unmarshal(&c)
	if err != nil {
		return nil, err
	}

	return &c, nil
}

func InitTimeZone() {
	ict, err := time.LoadLocation("Asia/Bangkok")
	if err != nil {
		panic(err)
	}
	time.Local = ict
}
