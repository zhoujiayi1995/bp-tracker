package profile

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"bptracker/internal/authz"
	"bptracker/internal/db"
	"bptracker/internal/validate"
)

func setup(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "bp.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func mustExec(t *testing.T, d *sql.DB, q string, args ...any) int64 {
	t.Helper()
	res, err := d.Exec(q, args...)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return id
}

func TestListVisibleAndTargets(t *testing.T) {
	ctx := context.Background()
	d := setup(t)
	now := time.Now()

	alice := mustExec(t, d, `INSERT INTO users (username, password_hash, created_at) VALUES ('alice', 'x', 0)`)
	bob := mustExec(t, d, `INSERT INTO users (username, password_hash, created_at) VALUES ('bob', 'x', 0)`)
	carol := mustExec(t, d, `INSERT INTO users (username, password_hash, created_at) VALUES ('carol', 'x', 0)`)
	fam := mustExec(t, d, `INSERT INTO families (name, invite_code, created_by, created_at) VALUES ('f', '123456', ?, 0)`, alice)
	mustExec(t, d, `UPDATE users SET family_id = ? WHERE id IN (?, ?)`, fam, alice, bob)

	aliceSelf, err := CreateSelf(ctx, d, alice, "Alice", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateSelf(ctx, d, bob, "Bob", now); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateSelf(ctx, d, carol, "Carol", now); err != nil {
		t.Fatal(err)
	}
	grandma, err := CreateManaged(ctx, d, alice, Fields{DisplayName: "Grandma", Gender: "female", BirthYear: 1945}, now)
	if err != nil {
		t.Fatal(err)
	}

	list, err := ListVisible(ctx, d, authz.Subject{UserID: alice, FamilyID: fam})
	if err != nil {
		t.Fatal(err)
	}
	gotNames := names(list)
	if want := []string{"Alice", "Grandma", "Bob"}; !equal(gotNames, want) {
		t.Fatalf("alice sees %v, want %v", gotNames, want)
	}

	bobList, _ := ListVisible(ctx, d, authz.Subject{UserID: bob, FamilyID: fam})
	if want := []string{"Bob", "Alice", "Grandma"}; !equal(names(bobList), want) {
		t.Fatalf("bob sees %v, want %v", names(bobList), want)
	}

	carolList, _ := ListVisible(ctx, d, authz.Subject{UserID: carol})
	if want := []string{"Carol"}; !equal(names(carolList), want) {
		t.Fatalf("carol sees %v, want %v", names(carolList), want)
	}

	g, err := Get(ctx, d, grandma)
	if err != nil {
		t.Fatal(err)
	}
	if !g.IsManaged() || g.ManagerName != "Alice" || g.OwnerFamilyID != fam || g.BirthYear != 1945 {
		t.Fatalf("managed profile = %+v", g)
	}
	bobSubject := authz.Subject{UserID: bob, FamilyID: fam}
	if !authz.CanRead(bobSubject, g.Target()) || authz.CanWrite(bobSubject, g.Target()) {
		t.Fatal("bob should read but not write grandma")
	}

	self, err := GetSelf(ctx, d, alice)
	if err != nil || self.ID != aliceSelf {
		t.Fatalf("GetSelf = %+v, %v", self, err)
	}
}

func TestUpdateDeleteAndValidation(t *testing.T) {
	ctx := context.Background()
	d := setup(t)
	now := time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC)
	alice := mustExec(t, d, `INSERT INTO users (username, password_hash, created_at) VALUES ('alice', 'x', 0)`)

	cases := []struct {
		f     Fields
		field string
	}{
		{Fields{DisplayName: "  "}, "displayName"},
		{Fields{DisplayName: "123456789012345678901"}, "displayName"},
		{Fields{DisplayName: "A", Gender: "x"}, "gender"},
		{Fields{DisplayName: "A", BirthYear: 1800}, "birthYear"},
		{Fields{DisplayName: "A", BirthYear: 2027}, "birthYear"},
	}
	for _, tc := range cases {
		_, err := CreateManaged(ctx, d, alice, tc.f, now)
		var ve *validate.Error
		if !errors.As(err, &ve) || ve.Field != tc.field {
			t.Errorf("CreateManaged(%+v) err = %v, want field %s", tc.f, err, tc.field)
		}
	}

	id, err := CreateManaged(ctx, d, alice, Fields{DisplayName: "Grandpa"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := Update(ctx, d, id, Fields{DisplayName: "Grandpa Li", Medications: " aspirin "}, now); err != nil {
		t.Fatal(err)
	}
	p, _ := Get(ctx, d, id)
	if p.DisplayName != "Grandpa Li" || p.Medications != "aspirin" {
		t.Fatalf("updated = %+v", p)
	}

	mustExec(t, d, `INSERT INTO readings (profile_id, measured_at, systolic, diastolic, created_by, created_at) VALUES (?, 0, 120, 80, ?, 0)`, id, alice)
	if err := Delete(ctx, d, id); err != nil {
		t.Fatal(err)
	}
	var n int
	_ = d.QueryRow(`SELECT COUNT(*) FROM readings WHERE profile_id = ?`, id).Scan(&n)
	if n != 0 {
		t.Fatalf("readings not cascaded, %d left", n)
	}
	if err := Delete(ctx, d, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete twice err = %v", err)
	}
}

func names(list []Profile) []string {
	out := make([]string, len(list))
	for i, p := range list {
		out[i] = p.DisplayName
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
