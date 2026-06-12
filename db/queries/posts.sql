-- name: CreatePost :one
INSERT INTO posts (user_id, body)
VALUES ($1, $2)
RETURNING id, user_id, body, created_at, updated_at;

-- name: GetPost :one
SELECT id, user_id, body, created_at, updated_at
FROM posts
WHERE id = $1;

-- name: GetPostForUser :one
SELECT id, user_id, body, created_at, updated_at
FROM posts
WHERE id = $1 AND user_id = $2;

-- name: ListPosts :many
SELECT id, user_id, body, created_at, updated_at
FROM posts
ORDER BY created_at DESC
LIMIT $1;

-- name: ListPostsByUser :many
SELECT id, user_id, body, created_at, updated_at
FROM posts
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2;

-- name: UpdatePostForUser :one
UPDATE posts
SET body = $3,
    updated_at = now()
WHERE id = $1 AND user_id = $2
RETURNING id, user_id, body, created_at, updated_at;

-- name: DeletePostForUser :exec
DELETE FROM posts
WHERE id = $1 AND user_id = $2;
