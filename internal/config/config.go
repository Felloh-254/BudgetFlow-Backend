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
	log.Println("[config] Load: loading configuration...")

	if err := godotenv.Load("../../.env"); err != nil {
		log.Println("[config] Load: no .env file found, relying on environment variables")
	} else {
		log.Println("[config] Load: .env file loaded successfully")
	}

	cfg := &Config{
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: mustGetEnv("DATABASE_URL"),
		JWTSecret:   mustGetEnv("JWT_SECRET"),
		CORSOrigins: strings.Split(getEnv("CORS_ORIGINS", "http://localhost:5173 "), ","),
	}

	hours, err := strconv.Atoi(getEnv("JWT_EXPIRY_HOURS", "24"))
	if err != nil || hours <= 0 {
		log.Printf("[config] Load: invalid JWT_EXPIRY_HOURS=%q, defaulting to 24", getEnv("JWT_EXPIRY_HOURS", "24"))
		hours = 24 // 1 day
	}
	cfg.JWTExpiry = time.Duration(hours) * time.Hour

	log.Printf("[config] Load: OK port=%s jwt_expiry=%s cors_origins=%v db_url_set=%v jwt_secret_set=%v",
		cfg.Port, cfg.JWTExpiry, cfg.CORSOrigins, cfg.DatabaseURL != "", cfg.JWTSecret != "")

	return cfg
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		log.Printf("[config] getEnv: %s is set", key)
		return v
	}
	log.Printf("[config] getEnv: %s not set, using fallback=%q", key, fallback)
	return fallback
}

func mustGetEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("[config] mustGetEnv: missing required environment variable: %s", key)
	}
	log.Printf("[config] mustGetEnv: %s is set", key)
	return v
}
