package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config contains PostgreSQL connection-pool settings.
// DatabaseURL is expected to be a PostgreSQL connection URI.
type Config struct {
	DatabaseURL string

	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
}

// DefaultConfig returns production-safe baseline pool settings.
func DefaultConfig(databaseURL string) Config {
	return Config{
		DatabaseURL:       databaseURL,
		MaxConns:          10,
		MinConns:          2,
		MaxConnLifetime:   time.Hour,
		MaxConnIdleTime:   30 * time.Minute,
		HealthCheckPeriod: time.Minute,
	}
}

// NewPool creates a PostgreSQL connection pool without performing a network
// connection. Call Ping when the application is ready to verify connectivity.
func NewPool(cfg Config) (*pgxpool.Pool, error) {
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("database URL is required")
	}
	if cfg.MaxConns <= 0 {
		return nil, fmt.Errorf("max connections must be greater than zero")
	}
	if cfg.MinConns < 0 {
		return nil, fmt.Errorf("min connections must not be negative")
	}
	if cfg.MinConns > cfg.MaxConns {
		return nil, fmt.Errorf("min connections must not exceed max connections")
	}
	if cfg.MaxConnLifetime <= 0 {
		return nil, fmt.Errorf("max connection lifetime must be greater than zero")
	}
	if cfg.MaxConnIdleTime <= 0 {
		return nil, fmt.Errorf("max connection idle time must be greater than zero")
	}
	if cfg.HealthCheckPeriod <= 0 {
		return nil, fmt.Errorf("health check period must be greater than zero")
	}

	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}

	poolConfig.MaxConns = cfg.MaxConns
	poolConfig.MinConns = cfg.MinConns
	poolConfig.MaxConnLifetime = cfg.MaxConnLifetime
	poolConfig.MaxConnIdleTime = cfg.MaxConnIdleTime
	poolConfig.HealthCheckPeriod = cfg.HealthCheckPeriod

	return pgxpool.NewWithConfig(context.Background(), poolConfig)
}

// Ping verifies that at least one usable database connection can be reached.
func Ping(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return fmt.Errorf("database pool is nil")
	}
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}
	return nil
}
