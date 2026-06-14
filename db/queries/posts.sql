-- name: CreatePost :one
INSERT INTO posts (user_id, body)
VALUES ($1, $2)
RETURNING id, user_id, body, created_at, updated_at;

-- name: CreateReply :one
WITH parent AS (
    SELECT posts.id, COALESCE(root_post_id, id) AS root_post_id
    FROM posts
    WHERE posts.id = $1
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
SELECT posts.id, posts.user_id, users.display_name, users.handle, posts.body, posts.created_at, posts.updated_at, count(replies.id)::bigint AS reply_count
FROM posts
JOIN users ON users.id = posts.user_id
LEFT JOIN posts AS replies ON replies.parent_post_id = posts.id
WHERE posts.parent_post_id IS NULL
GROUP BY posts.id, users.display_name, users.handle
ORDER BY posts.created_at DESC, posts.id DESC
LIMIT $1
OFFSET $2;

-- name: ListPostsByUser :many
SELECT posts.id, posts.user_id, users.display_name, users.handle, posts.body, posts.created_at, posts.updated_at, count(replies.id)::bigint AS reply_count
FROM posts
JOIN users ON users.id = posts.user_id
LEFT JOIN posts AS replies ON replies.parent_post_id = posts.id
WHERE posts.user_id = $1 AND posts.parent_post_id IS NULL
GROUP BY posts.id, users.display_name, users.handle
ORDER BY posts.created_at DESC, posts.id DESC
LIMIT $2
OFFSET $3;

-- name: ListPostReplies :many
SELECT posts.id, posts.user_id, users.display_name, users.handle, posts.body, posts.created_at, posts.updated_at, count(replies.id)::bigint AS reply_count
FROM posts
JOIN users ON users.id = posts.user_id
LEFT JOIN posts AS replies ON replies.parent_post_id = posts.id
WHERE posts.parent_post_id = $1
GROUP BY posts.id, users.display_name, users.handle
ORDER BY posts.created_at ASC, posts.id ASC
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
