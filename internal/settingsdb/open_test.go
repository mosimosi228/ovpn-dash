package settingsdb

import (
	"context"
	"os"
	"path/filepath"
	"testing"
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
