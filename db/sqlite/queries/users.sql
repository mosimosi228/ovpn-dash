-- name: CountUsers :one
SELECT COUNT(*) FROM users;

-- name: GetUserByID :one
SELECT id, email, name, pass_hash, role, client_name, telegram_chat_id, telegram_bind, theme, map_style, disabled, created_at, updated_at
FROM users WHERE id = ? LIMIT 1;

-- name: GetUserByEmail :one
SELECT id, email, name, pass_hash, role, client_name, telegram_chat_id, telegram_bind, theme, map_style, disabled, created_at, updated_at
FROM users WHERE email = ? LIMIT 1;

-- name: GetUserByClientName :one
SELECT id, email, name, pass_hash, role, client_name, telegram_chat_id, telegram_bind, theme, map_style, disabled, created_at, updated_at
FROM users WHERE client_name = ? LIMIT 1;

-- name: GetUserByTelegramChatID :one
SELECT id, email, name, pass_hash, role, client_name, telegram_chat_id, telegram_bind, theme, map_style, disabled, created_at, updated_at
FROM users WHERE telegram_chat_id = ? LIMIT 1;

-- name: GetUserByTelegramBind :one
SELECT id, email, name, pass_hash, role, client_name, telegram_chat_id, telegram_bind, theme, map_style, disabled, created_at, updated_at
FROM users WHERE telegram_bind = ? AND telegram_bind IS NOT NULL AND telegram_bind != '' LIMIT 1;

-- name: ListUsers :many
SELECT id, email, name, pass_hash, role, client_name, telegram_chat_id, telegram_bind, theme, map_style, disabled, created_at, updated_at
FROM users ORDER BY CASE role WHEN 'root' THEN 0 WHEN 'admin' THEN 1 ELSE 2 END, email;

-- name: InsertUser :execresult
INSERT INTO users (
  email, name, pass_hash, role, client_name, telegram_chat_id, telegram_bind,
  theme, map_style, disabled, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateUser :exec
UPDATE users SET
  email = ?,
  name = ?,
  pass_hash = ?,
  role = ?,
  client_name = ?,
  telegram_chat_id = ?,
  telegram_bind = ?,
  theme = ?,
  map_style = ?,
  disabled = ?,
  updated_at = ?
WHERE id = ?;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = ?;
