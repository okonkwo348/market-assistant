package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"market-assistant/internal/config"
	"market-assistant/internal/db"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{}))
	if cfg.AppEnv != "development" {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}))
	}

	pool, err := db.NewPool(db.DefaultConfig(cfg.DatabaseURL))
	if err != nil {
		logger.Error("database pool creation failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), cfg.DBPingTimeout)
	defer cancel()

	if err := db.Ping(ctx, pool); err != nil {
		logger.Error("database connectivity check failed", "error", err)
		os.Exit(1)
	}

	if err := db.Migrate(ctx, pool); err != nil {
		logger.Error("database migration failed", "error", err)
		os.Exit(1)
	}

	logger.Info("database migrations completed", "time", time.Now().UTC().Format(time.RFC3339))
}
