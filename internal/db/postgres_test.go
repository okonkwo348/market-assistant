package db

import (
	"context"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig("postgres://postgres:postgres@localhost:5432/market_assistant")

	if cfg.DatabaseURL == "" {
		t.Fatal("expected database URL")
	}
	if cfg.MaxConns != 10 {
		t.Fatalf("MaxConns = %d, want 10", cfg.MaxConns)
	}
	if cfg.MinConns != 2 {
		t.Fatalf("MinConns = %d, want 2", cfg.MinConns)
	}
	if cfg.MaxConnLifetime != time.Hour {
		t.Fatalf("MaxConnLifetime = %s, want 1h", cfg.MaxConnLifetime)
	}
}

func TestNewPoolValidation(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want string
	}{
		{
			name: "missing URL",
			cfg:  DefaultConfig(""),
			want: "database URL is required",
		},
		{
			name: "zero max connections",
			cfg: func() Config {
				cfg := DefaultConfig("postgres://localhost/db")
				cfg.MaxConns = 0
				return cfg
			}(),
			want: "max connections must be greater than zero",
		},
		{
			name: "min exceeds max",
			cfg: func() Config {
				cfg := DefaultConfig("postgres://localhost/db")
				cfg.MinConns = 11
				return cfg
			}(),
			want: "min connections must not exceed max connections",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewPool(tt.cfg)
			if err == nil || err.Error() != tt.want {
				t.Fatalf("NewPool() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestNewPoolRejectsInvalidDatabaseURL(t *testing.T) {
	cfg := DefaultConfig("not-a-postgres-url")

	pool, err := NewPool(cfg)
	if err == nil {
		pool.Close()
		t.Fatal("expected invalid database URL error")
	}
}

func TestPingNilPool(t *testing.T) {
	err := Ping(context.Background(), nil)
	if err == nil {
		t.Fatal("expected nil pool error")
	}
}
