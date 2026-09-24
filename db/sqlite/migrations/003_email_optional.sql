-- Existing installs created users.email NOT NULL. Rebuild so several clients
-- can omit email (SQLite UNIQUE allows multiple NULLs).
CREATE TABLE users_email_opt (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  email TEXT UNIQUE,
  name TEXT NOT NULL,
  pass_hash TEXT NOT NULL,
  role TEXT NOT NULL,
  client_name TEXT UNIQUE,
  telegram_chat_id TEXT,
  telegram_bind TEXT,
  theme TEXT NOT NULL DEFAULT 'light',
  map_style TEXT NOT NULL DEFAULT 'auto',
  disabled INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

INSERT INTO users_email_opt (
  id, email, name, pass_hash, role, client_name, telegram_chat_id, telegram_bind,
  theme, map_style, disabled, created_at, updated_at
)
SELECT
  id,
  NULLIF(email, ''),
  name, pass_hash, role, client_name, telegram_chat_id, telegram_bind,
  theme, map_style, disabled, created_at, updated_at
FROM users;

DROP TABLE users;
ALTER TABLE users_email_opt RENAME TO users;
