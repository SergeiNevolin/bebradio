package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	DatabaseURL   string
	SecretKey     string
	MediaServiceURL string
	CORSOrigins   []string

	JWTExpireHours int
	MaxDuration    int
	MaxChatMessages int

	AutoAdvanceInterval float64
	AutoAdvanceGrace    float64
	AdvanceDedupWindow  float64

	RadioRefillAt int
	RadioBatch    int

	RateLimitSearch int
	RateLimitQueue  int
	RateLimitWindow int

	Port string
}

func Load() *Config {
	return &Config{
		DatabaseURL:    getEnv("DATABASE_URL", "postgresql://postgres:postgres@localhost:5432/bebradio"),
		SecretKey:      getEnv("SECRET_KEY", "bebradio-secret-key-change-in-production"),
		MediaServiceURL: getEnv("MEDIA_SERVICE_URL", "http://127.0.0.1:8100"),
		CORSOrigins:    strings.Split(getEnv("CORS_ORIGINS", "http://localhost:3000"), ","),

		JWTExpireHours:  getEnvInt("JWT_EXPIRE_HOURS", 72),
		MaxDuration:     getEnvInt("MAX_DURATION", 3600),
		MaxChatMessages: 100,

		AutoAdvanceInterval: 2.0,
		AutoAdvanceGrace:    2.5,
		AdvanceDedupWindow:  1.0,

		RadioRefillAt: getEnvInt("RADIO_REFILL_AT", 1),
		RadioBatch:    getEnvInt("RADIO_BATCH", 3),

		RateLimitSearch: getEnvInt("RATE_LIMIT_SEARCH", 15),
		RateLimitQueue:  getEnvInt("RATE_LIMIT_QUEUE", 10),
		RateLimitWindow: getEnvInt("RATE_LIMIT_WINDOW", 60),

		Port: getEnv("PORT", "8000"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}
