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
	if err := godotenv.Load(".env"); err != nil {
		log.Println("[config] no .env file found, relying on environment variables")
	}

	cfg := &Config{
		Port:        mustGetEnv("PORT"),
		DatabaseURL: mustGetEnv("DATABASE_URL"),
		JWTSecret:   mustGetEnv("JWT_SECRET"),
		CORSOrigins: splitCSV(mustGetEnv("CORS_ORIGIN")),
		LogLevel:    mustGetEnv("LOG_LEVEL"),
		LogFormat:   mustGetEnv("LOG_FORMAT"),
	}

	raw := mustGetEnv("JWT_EXPIRY_HOURS")
	hours, err := strconv.Atoi(raw)
	if err != nil || hours <= 0 {
		hours = 24
	}
	cfg.JWTExpiry = time.Duration(hours) * time.Hour

	return cfg
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
