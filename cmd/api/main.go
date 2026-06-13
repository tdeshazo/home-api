package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/tdeshazo/home-api/internal/api"
	"github.com/tdeshazo/home-api/internal/db"
	"github.com/tdeshazo/home-api/internal/logging"

	_ "github.com/lib/pq"
)

func main() {
	os.Exit(run())
}

func run() int {
	logLevel, err := parseLogLevel(getenv("LOG_LEVEL", "info"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "configure logger: %v\n", err)
		return 1
	}

	logger, closeLogs, err := logging.New(logging.Options{
		Level:     logLevel,
		File:      os.Getenv("LOG_FILE"),
		Env:       getenv("APP_ENV", "development"),
		AddSource: getenv("LOG_SOURCE", "") == "true",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "configure logger: %v\n", err)
		return 1
	}
	defer func() {
		if err := closeLogs(); err != nil {
			fmt.Fprintf(os.Stderr, "close logs: %v\n", err)
		}
	}()

	databaseURL := getenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/social_api?sslmode=disable")
	port := getenv("PORT", "8080")
	accessTokenTTL, err := parseDuration("AUTH_ACCESS_TOKEN_TTL", 15*time.Minute)
	if err != nil {
		logger.Error("configure auth token ttl", "error", err)
		return 1
	}
	refreshTokenTTL, err := parseDuration("AUTH_REFRESH_TOKEN_TTL", 720*time.Hour)
	if err != nil {
		logger.Error("configure auth refresh ttl", "error", err)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sqlDB, err := sql.Open("postgres", databaseURL)
	if err != nil {
		logger.Error("connect postgres", "error", err)
		return 1
	}
	defer sqlDB.Close()

	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)
	sqlDB.SetConnMaxIdleTime(1 * time.Minute)

	if err := sqlDB.PingContext(ctx); err != nil {
		logger.Error("ping postgres", "error", err)
		return 1
	}

	queries := db.New(sqlDB)
	authConfig := api.AuthConfig{
		Environment:     getenv("APP_ENV", "development"),
		Mode:            getenv("AUTH_MODE", "dev"),
		JWTSecret:       os.Getenv("JWT_SECRET"),
		JWTIssuer:       os.Getenv("JWT_ISSUER"),
		JWTAudience:     os.Getenv("JWT_AUDIENCE"),
		AccessTokenTTL:  accessTokenTTL,
		RefreshTokenTTL: refreshTokenTTL,
	}
	auth, err := api.NewAuthenticator(queries, authConfig)
	if err != nil {
		logger.Error("configure auth", "error", err)
		return 1
	}

	server := api.NewServer(queries, sqlDB, logger, auth, authConfig)

	httpServer := &http.Server{
		Addr:              ":" + port,
		Handler:           server.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	shutdownSignal, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	logger.Info("listening", "addr", httpServer.Addr)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	select {
	case err := <-serverErr:
		if err != nil {
			logger.Error("listen", "error", err)
			return 1
		}
	case <-shutdownSignal.Done():
		stop()
		logger.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			logger.Error("shutdown", "error", err)
			return 1
		}
		if err := <-serverErr; err != nil {
			logger.Error("listen", "error", err)
			return 1
		}
	}

	return 0
}

func getenv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func parseLogLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug, nil
	case "", "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unsupported LOG_LEVEL %q; expected debug, info, warn, or error", raw)
	}
}

func parseDuration(key string, fallback time.Duration) (time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}

	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	if value <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero", key)
	}
	return value, nil
}
