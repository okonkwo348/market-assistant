package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadUsesEnvironment(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("APP_NAME", "test-app")
	t.Setenv("PORT", "9090")
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("HTTP_READ_TIMEOUT", "2s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.AppEnv != "test" {
		t.Errorf("AppEnv = %q, want %q", cfg.AppEnv, "test")
	}
	if cfg.AppName != "test-app" {
		t.Errorf("AppName = %q, want %q", cfg.AppName, "test-app")
	}
	if cfg.Port != "9090" {
		t.Errorf("Port = %q, want %q", cfg.Port, "9090")
	}
	if cfg.DatabaseURL != "postgres://example" {
		t.Errorf("DatabaseURL = %q, want configured value", cfg.DatabaseURL)
	}
	if cfg.HTTPReadTimeout != 2*time.Second {
		t.Errorf("HTTPReadTimeout = %v, want 2s", cfg.HTTPReadTimeout)
	}
}

func TestLoadRejectsInvalidDuration(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("HTTP_READ_TIMEOUT", "not-a-duration")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}

	if !strings.Contains(err.Error(), "HTTP_READ_TIMEOUT must be a valid duration") {
		t.Errorf("Load() error = %q, want duration validation error", err)
	}
}

func TestValidateRejectsInvalidConfiguration(t *testing.T) {
	cfg := Config{
		AppName: "market-assistant",
		Port:    "0",
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}

	message := err.Error()
	for _, expected := range []string{
		"DATABASE_URL is required",
		"PORT must be an integer between 1 and 65535",
	} {
		if !strings.Contains(message, expected) {
			t.Errorf("Validate() error = %q, want it to contain %q", message, expected)
		}
	}
}

func TestGetEnvUsesFallback(t *testing.T) {
	const key = "MARKET_ASSISTANT_TEST_ENV"
	t.Setenv(key, "")

	got := getEnv(key, "fallback")
	if got != "fallback" {
		t.Errorf("getEnv() = %q, want %q", got, "fallback")
	}

	t.Setenv(key, "configured")
	got = getEnv(key, "fallback")
	if got != "configured" {
		t.Errorf("getEnv() = %q, want %q", got, "configured")
	}
}
