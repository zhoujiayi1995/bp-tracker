package db

import (
	"context"
	"database/sql"
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

func TestMigrationPreservesRecordsAndPreventsIDReuse(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "bp.db")
	old, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { old.Close() })
	init, err := migrationFS.ReadFile("migrations/0001_init.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := old.Exec(string(init) + `
		CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at INTEGER NOT NULL);
		INSERT INTO schema_migrations VALUES (1, 1);
		INSERT INTO users (id, username, password_hash, created_at) VALUES (3, 'fixture', 'hash', 1);
		INSERT INTO profiles (id, display_name, manager_user_id, created_at) VALUES (7, 'Parent', 3, 1);
		INSERT INTO readings (id, profile_id, measured_at, systolic, diastolic, note, created_by, created_at)
			VALUES (12, 7, 1, 123, 80, 'After lunch', 3, 1);
		INSERT INTO reading_items (reading_id, seq, systolic, diastolic) VALUES (12, 1, 123, 80);
	`); err != nil {
		t.Fatal(err)
	}
	if err := old.Close(); err != nil {
		t.Fatal(err)
	}

	d, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	var name, note string
	var systolic int
	err = d.QueryRow(`
		SELECT p.display_name, r.note, i.systolic
		FROM profiles p JOIN readings r ON r.profile_id = p.id
		JOIN reading_items i ON i.reading_id = r.id
		WHERE p.id = 7 AND r.id = 12 AND i.seq = 1`).Scan(&name, &note, &systolic)
	if err != nil || name != "Parent" || note != "After lunch" || systolic != 123 {
		t.Fatalf("preserved data = %q, %q, %d, err %v", name, note, systolic, err)
	}

	var profileID, readingID, oldItems int
	if _, err = d.Exec(`DELETE FROM profiles WHERE id = 7`); err != nil {
		t.Fatal(err)
	}
	if err = d.QueryRow(`SELECT COUNT(*) FROM reading_items`).Scan(&oldItems); err != nil || oldItems != 0 {
		t.Fatalf("items after cascade = %d, err %v", oldItems, err)
	}
	err = d.QueryRow(`INSERT INTO profiles (display_name, manager_user_id, created_at)
		VALUES ('Another parent', 3, 1) RETURNING id`).Scan(&profileID)
	if err != nil || profileID <= 7 {
		t.Fatalf("replacement profile ID = %d, err %v", profileID, err)
	}
	err = d.QueryRow(`INSERT INTO readings (profile_id, measured_at, systolic, diastolic, created_by, created_at)
		VALUES (?, 1, 120, 80, 3, 1) RETURNING id`, profileID).Scan(&readingID)
	if err != nil || readingID <= 12 {
		t.Fatalf("replacement reading ID = %d, err %v", readingID, err)
	}
}
