package db

import (
	"context"
	"path/filepath"
	"testing"
)

func TestOpenAppliesMigrationsIdempotently(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "nested", "bp.db")

	first, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	first.Close()

	second, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	defer second.Close()

	var n int
	if err := second.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if want := len(mustList(t)); n != want {
		t.Fatalf("applied migrations = %d, want %d", n, want)
	}

	var mode string
	if err := second.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if mode != "wal" {
		t.Fatalf("journal_mode = %q, want wal", mode)
	}

	var fk int
	if err := second.QueryRow(`PRAGMA foreign_keys`).Scan(&fk); err != nil {
		t.Fatal(err)
	}
	if fk != 1 {
		t.Fatalf("foreign_keys = %d, want 1", fk)
	}
}

func mustList(t *testing.T) []migration {
	t.Helper()
	m, err := listMigrations()
	if err != nil {
		t.Fatal(err)
	}
	return m
}
