package family

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"bptracker/internal/db"
)

func setup(t *testing.T) (*sql.DB, func(name string) int64) {
	t.Helper()
	d, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "bp.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	newUser := func(name string) int64 {
		res, err := d.Exec(`INSERT INTO users (username, password_hash, created_at) VALUES (?, 'x', 0)`, name)
		if err != nil {
			t.Fatal(err)
		}
		id, _ := res.LastInsertId()
		return id
	}
	return d, newUser
}

func TestCreateJoinLeave(t *testing.T) {
	ctx := context.Background()
	d, newUser := setup(t)
	alice, bob := newUser("alice"), newUser("bob")

	f, err := Create(ctx, d, alice, " Home ", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if f.Name != "Home" || len(f.InviteCode) != inviteCodeLen {
		t.Fatalf("created family = %+v", f)
	}
	if _, err := Create(ctx, d, alice, "Again", time.Now()); !errors.Is(err, ErrAlreadyInFamily) {
		t.Fatalf("second create err = %v", err)
	}

	if _, err := Join(ctx, d, bob, "000000x"); !errors.Is(err, ErrInvalidInviteCode) {
		t.Fatalf("malformed code err = %v", err)
	}
	joined, err := Join(ctx, d, bob, f.InviteCode)
	if err != nil || joined.ID != f.ID {
		t.Fatalf("join = %+v, %v", joined, err)
	}

	members, err := Members(ctx, d, f.ID)
	if err != nil || len(members) != 2 {
		t.Fatalf("members = %+v, %v", members, err)
	}

	if err := Leave(ctx, d, alice); err != nil {
		t.Fatal(err)
	}
	if _, err := Get(ctx, d, f.ID); err != nil {
		t.Fatalf("family should survive while bob remains: %v", err)
	}
	if err := Leave(ctx, d, bob); err != nil {
		t.Fatal(err)
	}
	if _, err := Get(ctx, d, f.ID); !errors.Is(err, ErrNotInFamily) {
		t.Fatalf("empty family should be deleted, err = %v", err)
	}
	if err := Leave(ctx, d, bob); !errors.Is(err, ErrNotInFamily) {
		t.Fatalf("leave twice err = %v", err)
	}
}

func TestResetInviteCodeInvalidatesOld(t *testing.T) {
	ctx := context.Background()
	d, newUser := setup(t)
	alice, bob := newUser("alice"), newUser("bob")

	f, err := Create(ctx, d, alice, "Home", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	code, err := ResetInviteCode(ctx, d, alice)
	if err != nil {
		t.Fatal(err)
	}
	if code == f.InviteCode {
		t.Skip("random code collided with the previous one; extremely unlikely")
	}
	if _, err := Join(ctx, d, bob, f.InviteCode); !errors.Is(err, ErrInvalidInviteCode) {
		t.Fatalf("old code err = %v", err)
	}
	if _, err := Join(ctx, d, bob, code); err != nil {
		t.Fatalf("new code: %v", err)
	}
	if _, err := ResetInviteCode(ctx, d, newUser("carol")); !errors.Is(err, ErrNotInFamily) {
		t.Fatalf("reset without family err = %v", err)
	}
}
