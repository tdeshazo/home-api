DATABASE_URL ?= postgres://postgres:postgres@localhost:5432/social_api?sslmode=disable

.PHONY: docker-up docker-down migrate-up migrate-down seed sqlc run tidy

docker-up:
	docker compose up -d

docker-down:
	docker compose down

migrate-up:
	psql "$(DATABASE_URL)" -f db/migrations/000001_create_users_and_posts.up.sql

migrate-down:
	psql "$(DATABASE_URL)" -f db/migrations/000001_create_users_and_posts.down.sql

seed:
	psql "$(DATABASE_URL)" -f db/seed.sql

sqlc:
	sqlc generate

tidy:
	go mod tidy

run:
	go run ./cmd/api
