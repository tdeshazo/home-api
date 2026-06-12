-- name: CreateUser :one
INSERT INTO users (email, handle, display_name, password_hash)
VALUES ($1, $2, $3, $4)
RETURNING id, email, handle, display_name, password_hash, created_at, updated_at;

-- name: GetUser :one
SELECT id, email, handle, display_name, password_hash, created_at, updated_at
FROM users
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT id, email, handle, display_name, password_hash, created_at, updated_at
FROM users
WHERE email = $1;
