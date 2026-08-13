-- name: GetMeta :one
SELECT value FROM kv WHERE key = ? LIMIT 1;

-- name: SetMeta :exec
INSERT INTO kv (key, value) VALUES (?, ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value;

-- name: DeleteMeta :exec
DELETE FROM kv WHERE key = ?;

-- name: ListMeta :many
SELECT key, value FROM kv ORDER BY key;
