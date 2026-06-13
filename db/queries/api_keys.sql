-- name: CreateAPIKey :one
INSERT INTO api_keys (
    user_id,
    name,
    key_hash
) VALUES (
    $1,
    $2,
    $3
)
RETURNING id, user_id, name, created_at, updated_at, last_used_at, revoked_at;

-- name: TouchAPIKey :one
UPDATE api_keys
SET last_used_at = now()
WHERE key_hash = $1
  AND revoked_at IS NULL
RETURNING user_id;
