package settingsdb

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mosimosi228/ovpn-dash/internal/settingsdb/sqlitedb"
	_ "github.com/mutecomm/go-sqlcipher/v4" // SQLCipher driver
)

// DB wraps SQLCipher connection + sqlc queries.
type DB struct {
	SQL  *sql.DB
	Q    *sqlitedb.Queries
	Path string
}

// Open opens or creates an encrypted settings database under dir.
// Key from OVPNDASH_DATA_KEY or dir/data.key (autogen 0600).
func Open(dir string) (*DB, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	key, err := loadOrCreateKey(dir)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "settings.db")
	dsn := fmt.Sprintf("file:%s?_pragma_key=x'%s'&_pragma_cipher_page_size=4096",
		filepath.ToSlash(path), key)
	sqlDB, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)
	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("sqlcipher ping: %w", err)
	}
	if err := migrate(sqlDB); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return &DB{SQL: sqlDB, Q: sqlitedb.New(sqlDB), Path: path}, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS kv (
  key   TEXT PRIMARY KEY NOT NULL,
  value TEXT NOT NULL
);
`)
	return err
}

func loadOrCreateKey(dir string) (string, error) {
	if k := strings.TrimSpace(os.Getenv("OVPNDASH_DATA_KEY")); k != "" {
		return normalizeKeyHex(k), nil
	}
	path := filepath.Join(dir, "data.key")
	if b, err := os.ReadFile(path); err == nil {
		return normalizeKeyHex(strings.TrimSpace(string(b))), nil
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	hexKey := hex.EncodeToString(raw)
	if err := os.WriteFile(path, []byte(hexKey+"\n"), 0o600); err != nil {
		return "", err
	}
	return hexKey, nil
}

func normalizeKeyHex(k string) string {
	k = strings.TrimSpace(k)
	k = strings.TrimPrefix(k, "x'")
	k = strings.TrimSuffix(k, "'")
	if len(k) == 64 {
		if _, err := hex.DecodeString(k); err == nil {
			return k
		}
	}
	sum := sha256.Sum256([]byte(k))
	return hex.EncodeToString(sum[:])
}

// Close closes the DB.
func (d *DB) Close() error {
	if d == nil || d.SQL == nil {
		return nil
	}
	return d.SQL.Close()
}

// GetMeta returns meta value or empty.
func (d *DB) GetMeta(ctx context.Context, key string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	v, err := d.Q.GetMeta(ctx, key)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return v, err
}

// SetMeta upserts a meta key.
func (d *DB) SetMeta(ctx context.Context, key, value string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	return d.Q.SetMeta(ctx, sqlitedb.SetMetaParams{Key: key, Value: value})
}
