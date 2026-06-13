# social-api

A small Go + Postgres API scaffold that assumes `sqlc` owns database code generation.

It includes:

- Go `net/http` routing
- structured JSON logging and request middleware
- configurable dev/JWT auth middleware
- user-owned post authorization
- Postgres migrations and seed data
- `sqlc.yaml` for `pgx/v5`
- hand-written placeholder generated files under `internal/db` so the shape is clear before you run `sqlc generate`

## Project layout

```text
.
├── cmd
│   ├── api/main.go
│   └── token/main.go
├── db
│   ├── migrations
│   │   ├── 000001_create_users_and_posts.up.sql
│   │   └── 000001_create_users_and_posts.down.sql
│   ├── queries
│   │   ├── posts.sql
│   │   └── users.sql
│   └── seed.sql
├── internal
│   ├── api
│   │   ├── context.go
│   │   ├── handler_helpers.go
│   │   ├── health_handlers.go
│   │   ├── me_handlers.go
│   │   ├── json.go
│   │   ├── middleware.go
│   │   ├── post_handlers.go
│   │   └── server.go
│   └── db
│       ├── db.go
│       ├── models.go
│       ├── posts.sql.go
│       └── users.sql.go
├── docker-compose.yml
├── go.mod
├── Makefile
└── sqlc.yaml
```

## Prerequisites

Install:

- Go
- Docker
- Postgres `psql` CLI
- sqlc

```bash
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
```

## Run locally

```bash
make docker-up
make migrate-up
make seed
make sqlc
make run
```

The default database URL is:

```text
postgres://postgres:postgres@localhost:5432/social_api?sslmode=disable
```

## Logging

The API uses `internal/logging` for structured JSON logs. Each log includes build metadata, environment, and hostname fields.

Optional logging variables:

```text
LOG_LEVEL=info       # debug, info, warn, or error
LOG_FILE=            # when set, logs are written to stderr and appended to this file
LOG_SOURCE=false     # set true to include source file and line
```

## Auth modes

Auth is selected through environment variables:

```text
APP_ENV=development
AUTH_MODE=dev
```

Supported values:

```text
AUTH_MODE=dev  # local-only impersonation token
AUTH_MODE=jwt  # validates a signed JWT
```

The app refuses to start when `APP_ENV=production` and `AUTH_MODE=dev` are both set.

### Dev auth

For local development, the auth token is:

```text
Authorization: Bearer dev:<user_uuid>
```

Seed users:

```text
Alice: 00000000-0000-0000-0000-000000000001
Bob:   00000000-0000-0000-0000-000000000002
```

### JWT auth

For JWT mode, set:

```text
APP_ENV=production
AUTH_MODE=jwt
JWT_SECRET=change-me
JWT_ISSUER=social-api
JWT_AUDIENCE=social-api-api
```

This scaffold validates HS256 JWTs where the `sub` claim is the user UUID. For a production identity provider, prefer asymmetric signing such as RS256/ES256 and validate keys through JWKS.

Generate a local test JWT:

```bash
go run ./cmd/token   -user-id 00000000-0000-0000-0000-000000000001   -secret change-me   -issuer social-api   -audience social-api-api
```

Then call the API:

```bash
TOKEN="paste-generated-token-here"

curl   -H "Authorization: Bearer $TOKEN"   http://localhost:8080/me
```

## Example requests

Health check:

```bash
curl http://localhost:8080/healthz
```

List public posts:

```bash
curl http://localhost:8080/posts
```

Read the authenticated user:

```bash
curl \
  -H "Authorization: Bearer dev:00000000-0000-0000-0000-000000000001" \
  http://localhost:8080/me
```

Create a post as Alice:

```bash
curl -X POST http://localhost:8080/posts \
  -H "Authorization: Bearer dev:00000000-0000-0000-0000-000000000001" \
  -H "Content-Type: application/json" \
  -d '{"body":"Hello from Alice"}'
```

Update Alice's seeded post as Alice:

```bash
curl -X PATCH http://localhost:8080/posts/10000000-0000-0000-0000-000000000001 \
  -H "Authorization: Bearer dev:00000000-0000-0000-0000-000000000001" \
  -H "Content-Type: application/json" \
  -d '{"body":"Alice updated her own post"}'
```

Try to update Alice's post as Bob:

```bash
curl -X PATCH http://localhost:8080/posts/10000000-0000-0000-0000-000000000001 \
  -H "Authorization: Bearer dev:00000000-0000-0000-0000-000000000002" \
  -H "Content-Type: application/json" \
  -d '{"body":"Bob should not be allowed"}'
```

That request should return a `404` with `post not found or not owned by user`. Returning `404` instead of `403` is a common choice when you do not want to reveal whether another user's resource exists.

## Authorization pattern

For user-owned writes, queries are scoped by both the resource ID and authenticated user ID:

```sql
UPDATE posts
SET body = $3,
    updated_at = now()
WHERE id = $1 AND user_id = $2
RETURNING id, user_id, body, created_at, updated_at;
```

That protects against IDOR-style access where a user guesses another user's post ID.
