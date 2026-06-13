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

-- name: ListUsers :many
SELECT *
FROM users
ORDER BY display_name ASC, handle ASC;

-- name: UpdateUserPoints :one
UPDATE users
SET points = $2
WHERE id = $1
RETURNING *;

-- name: AddUserPoints :one
UPDATE users
SET points = points + $2
WHERE id = $1
RETURNING *;
