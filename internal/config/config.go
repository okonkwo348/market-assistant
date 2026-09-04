package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv      string
	AppName     string
	Port        string
	DatabaseURL string
	AuthSecret  string
	AuthTokenTTL time.Duration

	HTTPReadTimeout       time.Duration
	HTTPReadHeaderTimeout time.Duration
	HTTPWriteTimeout      time.Duration
	HTTPIdleTimeout       time.Duration
	HTTPShutdownTimeout   time.Duration
	DBPingTimeout         time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		AppEnv:      getEnv("APP_ENV", "development"),
		AppName:     getEnv("APP_NAME", "market-assistant"),
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: strings.TrimSpace(os.Getenv("DATABASE_URL")),
		AuthSecret:  strings.TrimSpace(os.Getenv("AUTH_SECRET")),
	}

	var err error
	cfg.AuthTokenTTL, err = getDuration("AUTH_TOKEN_TTL", 24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	cfg.HTTPReadTimeout, err = getDuration("HTTP_READ_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}
	cfg.HTTPReadHeaderTimeout, err = getDuration("HTTP_READ_HEADER_TIMEOUT", 5*time.Second)
	if err != nil {
		return Config{}, err
	}
	cfg.HTTPWriteTimeout, err = getDuration("HTTP_WRITE_TIMEOUT", 15*time.Second)
	if err != nil {
		return Config{}, err
	}
	cfg.HTTPIdleTimeout, err = getDuration("HTTP_IDLE_TIMEOUT", 60*time.Second)
	if err != nil {
		return Config{}, err
	}
	cfg.HTTPShutdownTimeout, err = getDuration("HTTP_SHUTDOWN_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}
	cfg.DBPingTimeout, err = getDuration("DB_PING_TIMEOUT", 5*time.Second)
	if err != nil {
		return Config{}, err
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) Validate() error {
	var errs []string

	if strings.TrimSpace(c.AppName) == "" {
		errs = append(errs, "APP_NAME must not be empty")
	}

	if strings.TrimSpace(c.DatabaseURL) == "" {
		errs = append(errs, "DATABASE_URL is required")
	}

	if len(strings.TrimSpace(c.AuthSecret)) < 32 {
		errs = append(errs, "AUTH_SECRET must be at least 32 characters")
	}

	if c.AuthTokenTTL <= 0 {
		errs = append(errs, "AUTH_TOKEN_TTL must be greater than zero")
	}

	port, err := strconv.Atoi(c.Port)
	if err != nil || port < 1 || port > 65535 {
		errs = append(errs, "PORT must be an integer between 1 and 65535")
	}

	if c.HTTPReadTimeout <= 0 {
		errs = append(errs, "HTTP_READ_TIMEOUT must be greater than zero")
	}
	if c.HTTPReadHeaderTimeout <= 0 {
		errs = append(errs, "HTTP_READ_HEADER_TIMEOUT must be greater than zero")
	}
	if c.HTTPWriteTimeout <= 0 {
		errs = append(errs, "HTTP_WRITE_TIMEOUT must be greater than zero")
	}
	if c.HTTPIdleTimeout <= 0 {
		errs = append(errs, "HTTP_IDLE_TIMEOUT must be greater than zero")
	}
	if c.HTTPShutdownTimeout <= 0 {
		errs = append(errs, "HTTP_SHUTDOWN_TIMEOUT must be greater than zero")
	}
	if c.DBPingTimeout <= 0 {
		errs = append(errs, "DB_PING_TIMEOUT must be greater than zero")
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}

	return nil
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func getDuration(key string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration: %w", key, err)
	}
	return duration, nil
}
