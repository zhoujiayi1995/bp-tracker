package auth

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"bptracker/internal/db"
	"bptracker/internal/validate"
)

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "bp.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestCreateAndAuthenticate(t *testing.T) {
	ctx := context.Background()
	d := openDB(t)
	now := time.Now()

	u, err := CreateUser(ctx, d, "alice", "secret1", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateUser(ctx, d, "ALICE", "secret1", now); !errors.Is(err, ErrUsernameTaken) {
		t.Fatalf("duplicate username (case-insensitive) err = %v, want ErrUsernameTaken", err)
	}

	got, err := Authenticate(ctx, d, "alice", "secret1")
	if err != nil || got.ID != u.ID {
		t.Fatalf("Authenticate = %+v, %v", got, err)
	}
	if _, err := Authenticate(ctx, d, "alice", "wrong"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password err = %v", err)
	}
	if _, err := Authenticate(ctx, d, "nobody", "secret1"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("unknown user err = %v", err)
	}
}

func TestCreateUserValidation(t *testing.T) {
	ctx := context.Background()
	d := openDB(t)
	cases := []struct {
		username, password, field string
	}{
		{"ab", "secret1", "username"},
		{"has space", "secret1", "username"},
		{"bob", "12345", "password"},
	}
	for _, tc := range cases {
		_, err := CreateUser(ctx, d, tc.username, tc.password, time.Now())
		var ve *validate.Error
		if !errors.As(err, &ve) || ve.Field != tc.field {
			t.Errorf("CreateUser(%q, %q) err = %v, want field %q", tc.username, tc.password, err, tc.field)
		}
	}
}

func TestChangePassword(t *testing.T) {
	ctx := context.Background()
	d := openDB(t)
	u, err := CreateUser(ctx, d, "alice", "secret1", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := ChangePassword(ctx, d, u.ID, "bad", "secret2"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong old password err = %v", err)
	}
	if err := ChangePassword(ctx, d, u.ID, "secret1", "secret2"); err != nil {
		t.Fatal(err)
	}
	if _, err := Authenticate(ctx, d, "alice", "secret2"); err != nil {
		t.Fatalf("login with new password: %v", err)
	}
}

func TestSessionLifecycle(t *testing.T) {
	ctx := context.Background()
	d := openDB(t)
	u, err := CreateUser(ctx, d, "alice", "secret1", time.Now())
	if err != nil {
		t.Fatal(err)
	}

	clock := time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)
	s := &Sessions{DB: d, Now: func() time.Time { return clock }}

	sess, err := s.Create(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}

	got, _, refreshed, err := s.Lookup(ctx, sess.Token)
	if err != nil || got.ID != u.ID || refreshed {
		t.Fatalf("fresh lookup = %+v refreshed=%v err=%v", got, refreshed, err)
	}

	clock = clock.Add(40 * 24 * time.Hour)
	_, renewed, refreshed, err := s.Lookup(ctx, sess.Token)
	if err != nil || !refreshed || !renewed.ExpiresAt.Equal(clock.Add(SessionTTL)) {
		t.Fatalf("sliding refresh: refreshed=%v expires=%v err=%v", refreshed, renewed.ExpiresAt, err)
	}

	clock = clock.Add(SessionTTL + time.Second)
	if _, _, _, err := s.Lookup(ctx, sess.Token); !errors.Is(err, ErrNoSession) {
		t.Fatalf("expired lookup err = %v", err)
	}

	sess2, _ := s.Create(ctx, u.ID)
	if err := s.Delete(ctx, sess2.Token); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := s.Lookup(ctx, sess2.Token); !errors.Is(err, ErrNoSession) {
		t.Fatalf("deleted lookup err = %v", err)
	}
}

func TestDeleteOthersKeepsCurrent(t *testing.T) {
	ctx := context.Background()
	d := openDB(t)
	u, _ := CreateUser(ctx, d, "alice", "secret1", time.Now())
	s := &Sessions{DB: d}
	keep, _ := s.Create(ctx, u.ID)
	drop, _ := s.Create(ctx, u.ID)

	if err := s.DeleteOthers(ctx, u.ID, keep.Token); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := s.Lookup(ctx, keep.Token); err != nil {
		t.Fatalf("kept session: %v", err)
	}
	if _, _, _, err := s.Lookup(ctx, drop.Token); !errors.Is(err, ErrNoSession) {
		t.Fatalf("other session err = %v", err)
	}
}

func TestLimiterLocksAfterMaxFailures(t *testing.T) {
	clock := time.Unix(0, 0)
	l := &Limiter{MaxFailures: 3, Window: time.Minute, Lockout: 5 * time.Minute, Now: func() time.Time { return clock }}

	for i := 0; i < 2; i++ {
		l.Fail("k")
	}
	if ok, _ := l.Allow("k"); !ok {
		t.Fatal("locked too early")
	}
	l.Fail("k")
	if ok, wait := l.Allow("k"); ok || wait != 5*time.Minute {
		t.Fatalf("Allow after max failures = %v, %v", ok, wait)
	}
	clock = clock.Add(5*time.Minute + time.Second)
	if ok, _ := l.Allow("k"); !ok {
		t.Fatal("still locked after lockout")
	}

	l.Fail("other")
	l.Reset("other")
	if ok, _ := l.Allow("other"); !ok {
		t.Fatal("reset did not clear key")
	}
}
