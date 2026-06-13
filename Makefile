DATABASE_URL ?= postgres://postgres:postgres@localhost:5432/social_api?sslmode=disable
BIN ?= bin/api
VERSION ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

.PHONY: build docker-up docker-down migrate-up migrate-down seed sqlc run tidy

docker-up:
	docker compose up -d

docker-down:
	docker compose down

migrate-up:
	psql "$(DATABASE_URL)" -f db/migrations/000001_create_users_and_posts.up.sql
	psql "$(DATABASE_URL)" -f db/migrations/000002_create_tasks.up.sql
	psql "$(DATABASE_URL)" -f db/migrations/000003_create_api_keys_refresh_tokens.up.sql
	psql "$(DATABASE_URL)" -f db/migrations/000004_alter_posts_threads.up.sql

migrate-down:
	psql "$(DATABASE_URL)" -f db/migrations/000004_alter_posts_threads.down.sql
	psql "$(DATABASE_URL)" -f db/migrations/000003_create_api_keys_refresh_tokens.down.sql
	psql "$(DATABASE_URL)" -f db/migrations/000002_create_tasks.down.sql
	psql "$(DATABASE_URL)" -f db/migrations/000001_create_users_and_posts.down.sql

seed:
	psql "$(DATABASE_URL)" -f db/seed.sql

sqlc:
	sqlc generate

tidy:
	go mod tidy

run:
	go run ./cmd/api

build:
	mkdir -p $(dir $(BIN))
	go build \
		-trimpath \
		-ldflags="-s -w -X github.com/tdeshazo/home-api/internal/build.GitSHA=$(VERSION) -X github.com/tdeshazo/home-api/internal/build.BuildTime=$(BUILD_TIME)" \
		-o $(BIN) \
		./cmd/api
