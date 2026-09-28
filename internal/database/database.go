// Package database sets up the Postgres connection pool. Kept separate
// from config so the pool itself (timeouts, size limits, health checks)
// is configured in one obvious place.
package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(databaseURL string) (*pgxpool.Pool, error) {
	log.Println("[database] NewPool: parsing database URL...")

	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		log.Printf("[database] NewPool: FAILED to parse database url: %v", err)
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	cfg.MaxConns = 10
	cfg.MinConns = 2
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 30 * time.Minute
	cfg.HealthCheckPeriod = time.Minute

	log.Printf("[database] NewPool: pool config max_conns=%d min_conns=%d max_lifetime=%s max_idle=%s health_check=%s",
		cfg.MaxConns, cfg.MinConns, cfg.MaxConnLifetime, cfg.MaxConnIdleTime, cfg.HealthCheckPeriod)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		log.Printf("[database] NewPool: FAILED to create pool: %v", err)
		return nil, fmt.Errorf("create pool: %w", err)
	}

	log.Println("[database] NewPool: pool created, pinging database...")

	if err := pool.Ping(ctx); err != nil {
		log.Printf("[database] NewPool: FAILED to ping database: %v", err)
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	log.Println("[database] NewPool: OK database connection established")
	return pool, nil
}
