-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (
    token_hash,
    user_id,
    expires_at
)
VALUES (
    $1,
    $2,
    $3
)
RETURNING token_hash, user_id, expires_at, revoked_at, created_at, updated_at;

-- name: GetRefreshToken :one
SELECT token_hash, user_id, expires_at, revoked_at, created_at, updated_at
FROM refresh_tokens
WHERE token_hash = $1;

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens
SET
    revoked_at = NOW()
WHERE
    token_hash = $1
    AND revoked_at IS NULL;
