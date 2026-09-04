package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"market-assistant/internal/auth"
	"market-assistant/internal/businesses"
	"market-assistant/internal/config"
	"market-assistant/internal/db"
	"market-assistant/internal/httpapi"
	"market-assistant/internal/users"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(1)
	}

	logger := newLogger(cfg.AppEnv)
	slog.SetDefault(logger)

	dbPool, err := db.NewPool(db.DefaultConfig(cfg.DatabaseURL))
	if err != nil {
		logger.Error("database pool creation failed", "error", err)
		os.Exit(1)
	}
	defer dbPool.Close()

	pingCtx, cancelPing := context.WithTimeout(context.Background(), cfg.DBPingTimeout)
	if err := db.Ping(pingCtx, dbPool); err != nil {
		cancelPing()
		logger.Error("database connectivity check failed", "error", err)
		os.Exit(1)
	}
	cancelPing()
	logger.Info("database connection ready")

	userRepository, err := users.NewPostgresRepository(dbPool)
	if err != nil {
		logger.Error("user repository creation failed", "error", err)
		os.Exit(1)
	}
	userService, err := users.NewService(userRepository)
	if err != nil {
		logger.Error("user service creation failed", "error", err)
		os.Exit(1)
	}

	businessRepository, err := businesses.NewPostgresRepository(dbPool)
	if err != nil {
		logger.Error("business repository creation failed", "error", err)
		os.Exit(1)
	}
	businessService, err := businesses.NewService(businessRepository)
	if err != nil {
		logger.Error("business service creation failed", "error", err)
		os.Exit(1)
	}

	apiHandler, err := httpapi.NewHandler(businessService)
	if err != nil {
		logger.Error("http handler creation failed", "error", err)
		os.Exit(1)
	}

	tokenManager, err := auth.NewTokenManager(cfg.AuthSecret, cfg.AuthTokenTTL)
	if err != nil {
		logger.Error("authentication setup failed", "error", err)
		os.Exit(1)
	}

	verificationRepository, err := auth.NewPostgresVerificationRepository(dbPool)
	if err != nil {
		logger.Error("verification repository creation failed", "error", err)
		os.Exit(1)
	}
	verificationService, err := auth.NewVerificationService(
		verificationRepository,
		cfg.AuthSecret,
		cfg.AuthTokenTTL,
	)
	if err != nil {
		logger.Error("verification service creation failed", "error", err)
		os.Exit(1)
	}
	verificationSender := auth.NewLogVerificationCodeSender(logger)
	verificationFlow, err := auth.NewVerificationFlow(verificationService, verificationSender)
	if err != nil {
		logger.Error("verification flow creation failed", "error", err)
		os.Exit(1)
	}

	verificationHandler, err := auth.NewVerificationHandler(
		userService,
		verificationFlow,
		verificationService,
	)
	if err != nil {
		logger.Error("verification handler creation failed", "error", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", healthHandler)

	authenticated := func(next http.Handler) http.Handler {
		return tokenManager.Middleware(httpapi.RequireUser(next))
	}

	mux.Handle("GET /api/businesses", authenticated(http.HandlerFunc(apiHandler.ListBusinesses)))
	mux.Handle("GET /api/businesses/{businessID}", authenticated(http.HandlerFunc(apiHandler.GetBusiness)))
	mux.Handle("POST /api/auth/verification/request", authenticated(http.HandlerFunc(verificationHandler.RequestCode)))
	mux.Handle("POST /api/auth/verification/verify", authenticated(http.HandlerFunc(verificationHandler.VerifyCode)))

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           requestLogging(mux, logger),
		ReadTimeout:       cfg.HTTPReadTimeout,
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("http server starting",
			"app", cfg.AppName,
			"env", cfg.AppEnv,
			"addr", server.Addr,
		)

		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTPShutdownTimeout)
		defer cancel()

		logger.Info("shutting down http server", "reason", ctx.Err())

		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
			os.Exit(1)
		}

		logger.Info("http server stopped")
	case err := <-serverErr:
		logger.Error("http server failed", "error", err)
		os.Exit(1)
	}
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func requestLogging(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)

		logger.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", recorder.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func newLogger(env string) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}

	if env == "development" {
		return slog.New(slog.NewTextHandler(os.Stdout, opts))
	}

	return slog.New(slog.NewJSONHandler(os.Stdout, opts))
}
