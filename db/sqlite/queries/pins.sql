-- name: InsertPinChallenge :exec
INSERT INTO pin_challenges (user_id, pin_hash, expires_at, created_at)
VALUES (?, ?, ?, ?);

-- name: GetPinChallenge :one
SELECT id, user_id, pin_hash, expires_at, created_at
FROM pin_challenges WHERE user_id = ? ORDER BY id DESC LIMIT 1;

-- name: DeletePinChallenges :exec
DELETE FROM pin_challenges WHERE user_id = ?;

-- name: DeleteExpiredPinChallenges :exec
DELETE FROM pin_challenges WHERE expires_at < ?;
