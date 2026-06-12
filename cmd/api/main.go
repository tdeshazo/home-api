package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"social-api/internal/api"
	"social-api/internal/db"
	"social-api/internal/logging"

	"github.com/jackc/pgx/v5/pgxpool"
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		logger.Error("connect postgres", "error", err)
		return 1
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		logger.Error("ping postgres", "error", err)
		return 1
	}

	queries := db.New(pool)
	auth, err := api.NewAuthenticator(queries, api.AuthConfig{
		Environment: getenv("APP_ENV", "development"),
		Mode:        getenv("AUTH_MODE", "dev"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		JWTIssuer:   os.Getenv("JWT_ISSUER"),
		JWTAudience: os.Getenv("JWT_AUDIENCE"),
	})
	if err != nil {
		logger.Error("configure auth", "error", err)
		return 1
	}

	server := api.NewServer(queries, logger, auth)

	httpServer := &http.Server{
		Addr:              ":" + port,
		Handler:           server.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("listening", "addr", httpServer.Addr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("listen", "error", err)
		return 1
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
