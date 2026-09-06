package config

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"strconv"
	"strings"
	"time"
)

type Model struct {
	ID             string  `yaml:"id"`
	Capability     string  `yaml:"capability"`
	ThinkingLevel  string  `yaml:"thinking_level"`
	TimeoutSeconds int     `yaml:"timeout_seconds"`
	Input          float64 `yaml:"input"`
	AudioInput     float64 `yaml:"audio_input"`
	Output         float64 `yaml:"output"`
	AudioOutput    float64 `yaml:"audio_output"`
	MaxTokens      int     `yaml:"max_tokens"`
	Temperature    float64 `yaml:"temperature"`
	ThinkingBudget int     `yaml:"thinking_budget"`
	Fallback       string  `yaml:"fallback"`
}
type Config struct {
	CacheTTLSeconds     int              `yaml:"cache_ttl_seconds"`
	CacheMaxMB          int              `yaml:"cache_max_mb"`
	AudioLocalCacheDays int              `yaml:"audio_local_cache_days"`
	MinIO               MinIOConfig      `yaml:"MinIO"`
	Port                string           `yaml:"port"`
	DatabaseURL         string           `yaml:"-"`
	GeminiKey           string           `yaml:"-"`
	AudioDir            string           `yaml:"audio_dir"`
	PublicURL           string           `yaml:"public_url"`
	Origins             []string         `yaml:"origins"`
	Version             string           `yaml:"version"`
	PriceDate           string           `yaml:"price_date"`
	USDTHB              float64          `yaml:"usd_thb"`
	MonthlyBudget       float64          `yaml:"monthly_budget"`
	LiveMinutes         int              `yaml:"live_minutes"`
	RetentionDays       int              `yaml:"retention_days"`
	Voice               string           `yaml:"voice"`
	TimeoutSeconds      int              `yaml:"timeout_seconds"`
	Models              map[string]Model `yaml:"models"`
}

func Load() (Config, error) {
	c := Config{CacheTTLSeconds: 30, CacheMaxMB: 32, AudioLocalCacheDays: 3, MinIO: MinIOConfig{Bucket: "ai-tutor", PrefixTTS: "tts/", PrefixUserAudio: "user-audio/"}, Port: "8080", AudioDir: "./data/audio", PublicURL: "http://localhost:8080", Origins: []string{"http://localhost:3000"}, Version: "2026-09-05.2", PriceDate: "2026-09-05", USDTHB: 35, MonthlyBudget: 600, LiveMinutes: 10, RetentionDays: 30, Voice: "Kore", TimeoutSeconds: 45, Models: map[string]Model{
		"helper": {ID: "gemini-2.5-flash-lite", Input: .10, AudioInput: .30, Output: .40, MaxTokens: 512},
		"tutor":  {ID: "gemini-3.1-flash-lite", Input: .25, AudioInput: .50, Output: 1.50, MaxTokens: 2048, Temperature: .5},
		"tts":    {ID: "gemini-2.5-flash-preview-tts", Input: .50, Output: 10, AudioOutput: 10, MaxTokens: 4096, Temperature: 1, TimeoutSeconds: 75},
		"live":   {ID: "gemini-3.1-flash-live-preview", Input: .75, AudioInput: 3, Output: 4.5, AudioOutput: 12, MaxTokens: 1024},
	}}
	if p := os.Getenv("TOKO_CONFIG"); p != "" {
		b, e := os.ReadFile(p)
		if e != nil {
			return c, e
		}
		if e = yaml.Unmarshal(b, &c); e != nil {
			return c, e
		}
	}
	c.DatabaseURL = os.Getenv("DATABASE_URL")
	c.GeminiKey = os.Getenv("GEMINI_API_KEY")
	if v := os.Getenv("PORT"); v != "" {
		c.Port = v
	}
	if v := os.Getenv("AUDIO_DIR"); v != "" {
		c.AudioDir = v
	}
	if v := os.Getenv("PUBLIC_BACKEND_URL"); v != "" {
		c.PublicURL = strings.TrimRight(v, "/")
	}
	if v := os.Getenv("ALLOWED_ORIGINS"); v != "" {
		c.Origins = strings.Split(v, ",")
	}
	if v := os.Getenv("RELEASE_ID"); v != "" {
		c.Version = v
	}
	for name, dest := range map[string]*string{"MINIO_ENDPOINT": &c.MinIO.Endpoint, "MINIO_ACCESS_KEY": &c.MinIO.AccessKey, "MINIO_SECRET_KEY": &c.MinIO.SecretKey, "MINIO_BUCKET": &c.MinIO.Bucket, "MINIO_PREFIX_TTS": &c.MinIO.PrefixTTS, "MINIO_PREFIX_USER_AUDIO": &c.MinIO.PrefixUserAudio} {
		if v := os.Getenv(name); v != "" {
			*dest = v
		}
	}
	if v := os.Getenv("MINIO_USE_SSL"); v != "" {
		parsed, err := strconv.ParseBool(v)
		if err != nil {
			return c, fmt.Errorf("invalid MINIO_USE_SSL")
		}
		c.MinIO.UseSSL = parsed
	}
	for name, dest := range map[string]*int{"CACHE_TTL_SECONDS": &c.CacheTTLSeconds, "CACHE_MAX_MB": &c.CacheMaxMB, "AUDIO_LOCAL_CACHE_DAYS": &c.AudioLocalCacheDays} {
		if v := os.Getenv(name); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil {
				return c, fmt.Errorf("invalid %s", name)
			}
			*dest = n
		}
	}
	if c.CacheMaxMB < 1 || c.CacheTTLSeconds < 0 || c.AudioLocalCacheDays < 1 {
		return c, fmt.Errorf("invalid cache configuration")
	}
	if c.MinIO.Endpoint != "" && (c.MinIO.AccessKey == "" || c.MinIO.SecretKey == "" || c.MinIO.Bucket == "") {
		return c, fmt.Errorf("incomplete MinIO configuration")
	}
	if c.DatabaseURL == "" {
		return c, fmt.Errorf("DATABASE_URL must name a new Toko Loop database")
	}
	if c.MonthlyBudget <= 0 || c.MonthlyBudget > 1000 || c.USDTHB <= 0 || c.RetentionDays < 1 || c.LiveMinutes < 1 || c.TimeoutSeconds < 1 {
		return c, fmt.Errorf("invalid budget, exchange rate, retention or timeout")
	}
	if _, e := time.Parse("2006-01-02", c.PriceDate); e != nil {
		return c, fmt.Errorf("invalid price_date")
	}
	for _, role := range []string{"helper", "tutor", "tts", "live"} {
		m, ok := c.Models[role]
		expected := map[string]string{"helper": "text_audio", "tutor": "text_audio", "tts": "tts", "live": "live"}[role]
		if m.Capability == "" {
			m.Capability = expected
			c.Models[role] = m
		}
		if m.Capability != expected {
			return c, fmt.Errorf("capability mismatch for %s", role)
		}
		if !ok || !strings.HasPrefix(m.ID, "gemini-") || m.MaxTokens < 1 || m.Input < 0 || m.Output < 0 || m.AudioInput < 0 || m.AudioOutput < 0 {
			return c, fmt.Errorf("invalid Gemini profile: %s", role)
		}
		if m.Fallback != "" {
			found := false
			for _, f := range c.Models {
				if f.ID == m.Fallback && f.ID != m.ID && f.Capability == m.Capability && f.Input >= 0 && f.Output > 0 && f.MaxTokens > 0 {
					found = true
				}
			}
			if !strings.HasPrefix(m.Fallback, "gemini-") || !found {
				return c, fmt.Errorf("fallback for %s needs a separate priced model profile with the same capability", role)
			}
		}
		if role == "tts" && !strings.Contains(m.ID, "tts") {
			return c, fmt.Errorf("tts model must support speech generation")
		}
		if role == "live" && !strings.Contains(m.ID, "live") && !strings.Contains(m.ID, "native-audio") {
			return c, fmt.Errorf("live model must support Live API")
		}
	}
	if _, e := strconv.Atoi(c.Port); e != nil {
		return c, e
	}
	return c, nil
}

type MinIOConfig struct {
	Endpoint        string `yaml:"endpoint"`
	AccessKey       string `yaml:"access_key"`
	SecretKey       string `yaml:"secret_key"`
	Bucket          string `yaml:"bucket"`
	UseSSL          bool   `yaml:"use_ssl"`
	PrefixTTS       string `yaml:"prefix_tts"`
	PrefixUserAudio string `yaml:"prefix_user_audio"`
}
