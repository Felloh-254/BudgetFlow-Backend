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
}

func Load() *Config {
	// .env is a local dev convenience; in production (Render, Railway, Fly,
	// etc.) real environment variables are already set and this is a no-op.
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, relying on environment variables")
	}

	cfg := &Config{
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: mustGetEnv("DATABASE_URL"),
		JWTSecret:   mustGetEnv("JWT_SECRET"),
		CORSOrigins: strings.Split(getEnv("CORS_ORIGINS", "http://localhost:5173"), ","),
	}

	hours, err := strconv.Atoi(getEnv("JWT_EXPIRY_HOURS", "168"))
	if err != nil || hours <= 0 {
		hours = 168 // 7 days
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
		log.Fatalf("missing required environment variable: %s", key)
	}
	return v
}
