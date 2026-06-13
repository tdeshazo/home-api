-- name: CreateUser :one
INSERT INTO users (email, handle, display_name, password_hash)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetUser :one
SELECT *
FROM users
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT *
FROM users
WHERE email = $1;

-- name: UpdateUserPoints :one
UPDATE users
SET points = $2
WHERE id = $1
RETURNING *;
