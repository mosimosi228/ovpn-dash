package settingsdb

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mosimosi228/ovpn-dash/internal/setup"
)

func TestOpenGetSetMeta(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := os.Stat(filepath.Join(dir, "data.key")); err != nil {
		t.Fatal(err)
	}
	if err := db.SetMeta(context.Background(), "k", "v"); err != nil {
		t.Fatal(err)
	}
	got, err := db.GetMeta(context.Background(), "k")
	if err != nil || got != "v" {
		t.Fatalf("got %q %v", got, err)
	}
	empty, err := db.GetMeta(context.Background(), "missing")
	if err != nil || empty != "" {
		t.Fatalf("missing %q %v", empty, err)
	}
}

func TestMigrateAdminToRoot(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := db.SetMeta(ctx, setup.KeyAdminUser, "legacy"); err != nil {
		t.Fatal(err)
	}
	if err := db.SetMeta(ctx, setup.KeyAdminPassHash, "hash-from-v1"); err != nil {
		t.Fatal(err)
	}
	if err := db.SetMeta(ctx, setup.KeyPublicHost, "vpn.example.com"); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	db2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db2.Close() })
	u, err := db2.GetUserByEmail(ctx, "legacy@vpn.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if u.Role != setup.RoleRoot || u.PassHash != "hash-from-v1" || u.Name != "legacy" {
		t.Fatalf("migrated %+v", u)
	}
	n, _ := db2.CountUsers(ctx)
	if n != 1 {
		t.Fatalf("count %d", n)
	}
	// second open must not duplicate
	_ = db2.Close()
	db3, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db3.Close() })
	n, _ = db3.CountUsers(ctx)
	if n != 1 {
		t.Fatalf("dup count %d", n)
	}
}
