-- name: CreatePost :one
INSERT INTO posts (user_id, body)
VALUES ($1, $2)
RETURNING id, user_id, body, created_at, updated_at;

-- name: CreateReply :one
WITH parent AS (
    SELECT id, COALESCE(root_post_id, id) AS root_post_id
    FROM posts
    WHERE id = $1
)
INSERT INTO posts (user_id, body, parent_post_id, root_post_id)
SELECT $2, $3, parent.id, parent.root_post_id
FROM parent
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
WHERE parent_post_id IS NULL
ORDER BY created_at DESC, id DESC
LIMIT $1
OFFSET $2;

-- name: ListPostsByUser :many
SELECT id, user_id, body, created_at, updated_at
FROM posts
WHERE user_id = $1 AND parent_post_id IS NULL
ORDER BY created_at DESC, id DESC
LIMIT $2
OFFSET $3;

-- name: ListPostReplies :many
SELECT id, user_id, body, created_at, updated_at
FROM posts
WHERE parent_post_id = $1
ORDER BY created_at ASC, id ASC
LIMIT $2
OFFSET $3;

-- name: UpdatePostForUser :one
UPDATE posts
SET body = $3,
    updated_at = now()
WHERE id = $1 AND user_id = $2
RETURNING id, user_id, body, created_at, updated_at;

-- name: DeletePostForUser :exec
DELETE FROM posts
WHERE id = $1 AND user_id = $2;
