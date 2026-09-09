-- name: InsertRecoveryToken :exec
INSERT INTO recovery_tokens (user_id, token_hash, expires_at)
VALUES (?, ?, ?);

-- name: GetRecoveryToken :one
SELECT id, user_id, token_hash, expires_at
FROM recovery_tokens WHERE token_hash = ? LIMIT 1;

-- name: DeleteRecoveryToken :exec
DELETE FROM recovery_tokens WHERE id = ?;

-- name: DeleteRecoveryTokensForUser :exec
DELETE FROM recovery_tokens WHERE user_id = ?;

-- name: DeleteExpiredRecoveryTokens :exec
DELETE FROM recovery_tokens WHERE expires_at < ?;
