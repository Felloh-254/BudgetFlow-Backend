// Package config centralizes environment configuration. Nothing in the
// rest of the app calls os.Getenv directly — everything goes through this
// struct so config is explicit, testable, and validated at startup instead
// of failing deep inside a handler.
package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   string
	JWTExpiry   time.Duration
	CORSOrigins []string
	LogLevel    string
	LogFormat   string
}

func Load() *Config {
	// The configured logger doesn't exist yet (it needs this config), so
	// this package uses the stdlib log.
	if err := godotenv.Load("../../.env"); err != nil {
		log.Println("[config] no .env file found, relying on environment variables")
	}

	cfg := &Config{
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: mustGetEnv("DATABASE_URL"),
		JWTSecret:   mustGetEnv("JWT_SECRET"),
		CORSOrigins: splitCSV(getEnv("CORS_ORIGINS", "http://localhost:5173")),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
		LogFormat:   getEnv("LOG_FORMAT", "text"),
	}

	raw := getEnv("JWT_EXPIRY_HOURS", "24")
	hours, err := strconv.Atoi(raw)
	if err != nil || hours <= 0 {
		hours = 24
	}
	cfg.JWTExpiry = time.Duration(hours) * time.Hour

	return cfg
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustGetEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("[config] missing required environment variable: %s", key)
	}
	return v
}

// splitCSV splits a comma-separated list, trimming spaces and dropping
// empty entries ("a.com, b.com" -> ["a.com", "b.com"]).
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
