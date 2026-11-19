package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"bptracker/internal/db"
	readingpkg "bptracker/internal/reading"
)

type client struct {
	t    *testing.T
	base string
	http *http.Client
}

type response struct {
	status int
	body   map[string]any
	list   []any
}

func newTestServer(t *testing.T) string {
	t.Helper()
	base, _ := newTestServerWithDB(t)
	return base
}

func newTestServerWithDB(t *testing.T) (string, *sql.DB) {
	t.Helper()
	d, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "bp.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	loc, _ := time.LoadLocation("Asia/Shanghai")
	srv := New(Options{DB: d, Location: loc, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts.URL, d
}

func newClient(t *testing.T, base string) *client {
	jar, _ := cookiejar.New(nil)
	return &client{t: t, base: base, http: &http.Client{Jar: jar}}
}

func (c *client) do(method, path string, body any) response {
	return c.request(method, path, body, true)
}

func (c *client) request(method, path string, body any, withCSRFHeader bool) response {
	c.t.Helper()
	var reader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.base+"/api"+path, reader)
	if err != nil {
		c.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if withCSRFHeader {
		req.Header.Set("X-Requested-With", "test")
	}
	res, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	out := response{status: res.StatusCode}
	if len(raw) > 0 {
		if raw[0] == '[' {
			_ = json.Unmarshal(raw, &out.list)
		} else {
			_ = json.Unmarshal(raw, &out.body)
		}
	}
	return out
}

func (r response) errCode() string {
	e, _ := r.body["error"].(map[string]any)
	code, _ := e["code"].(string)
	return code
}

func expect(t *testing.T, r response, status int, what string) response {
	t.Helper()
	if r.status != status {
		t.Fatalf("%s: status = %d (%s), want %d; body=%v", what, r.status, r.errCode(), status, r.body)
	}
	return r
}

func num(v any) int64 {
	f, _ := v.(float64)
	return int64(f)
}

func TestEndToEndFamilyFlowAndAccessControl(t *testing.T) {
	base := newTestServer(t)
	alice, bob, carol := newClient(t, base), newClient(t, base), newClient(t, base)

	expect(t, newClient(t, base).do("GET", "/me", nil), 401, "anonymous /me")

	aliceMe := expect(t, alice.do("POST", "/auth/register", map[string]any{
		"username": "alice", "password": "secret1", "displayName": "Alice",
	}), 201, "register alice").body
	expect(t, bob.do("POST", "/auth/register", map[string]any{
		"username": "bob", "password": "secret1", "displayName": "Bob",
	}), 201, "register bob")
	expect(t, carol.do("POST", "/auth/register", map[string]any{
		"username": "carol", "password": "secret1", "displayName": "Carol",
	}), 201, "register carol")

	dup := expect(t, newClient(t, base).do("POST", "/auth/register", map[string]any{
		"username": "Alice", "password": "secret1", "displayName": "X",
	}), 409, "duplicate username")
	if dup.errCode() != "username_taken" {
		t.Fatalf("duplicate code = %s", dup.errCode())
	}

	aliceSelf := num(aliceMe["selfProfileId"])

	// Alice creates a family; Bob joins after one wrong guess.
	fam := expect(t, alice.do("POST", "/family", map[string]any{"name": "Home"}), 201, "create family").body
	code := fam["family"].(map[string]any)["inviteCode"].(string)
	wrong := "000000"
	if code == wrong {
		wrong = "111111"
	}
	if r := expect(t, bob.do("POST", "/family/join", map[string]any{"inviteCode": wrong}), 400, "wrong code"); r.errCode() != "invalid_invite_code" {
		t.Fatalf("wrong code err = %s", r.errCode())
	}
	joined := expect(t, bob.do("POST", "/family/join", map[string]any{"inviteCode": code}), 200, "join").body
	if n := len(joined["members"].([]any)); n != 2 {
		t.Fatalf("members after join = %d", n)
	}

	// Bob manages Grandma and records for her.
	grandma := expect(t, bob.do("POST", "/profiles", map[string]any{
		"displayName": "Grandma", "gender": "female", "birthYear": 1946, "medications": "amlodipine",
	}), 201, "create managed profile").body
	grandmaID := num(grandma["id"])

	reading := expect(t, bob.do("POST", fmt.Sprintf("/profiles/%d/readings", grandmaID), map[string]any{
		"items": []map[string]any{
			{"systolic": 150, "diastolic": 90, "pulse": 70},
			{"systolic": 145, "diastolic": 87, "pulse": 73},
		},
		"note": "after breakfast",
	}), 201, "bob records for grandma").body
	if num(reading["systolic"]) != 148 || num(reading["diastolic"]) != 89 || num(reading["pulse"]) != 72 {
		t.Fatalf("averaged reading = %v", reading)
	}
	readingID := num(reading["id"])

	// Alice sees Grandma read-only.
	profiles := expect(t, alice.do("GET", "/profiles", nil), 200, "alice profiles").list
	var sawGrandma bool
	for _, p := range profiles {
		pm := p.(map[string]any)
		if num(pm["id"]) == grandmaID {
			sawGrandma = true
			if pm["canWrite"] != false || pm["managerName"] != "Bob" {
				t.Fatalf("grandma as seen by alice = %v", pm)
			}
		}
	}
	if !sawGrandma || len(profiles) != 3 {
		t.Fatalf("alice should see 3 profiles incl. grandma, got %v", profiles)
	}
	list := expect(t, alice.do("GET", fmt.Sprintf("/profiles/%d/readings", grandmaID), nil), 200, "alice reads grandma").list
	if len(list) != 1 {
		t.Fatalf("alice sees %d grandma readings", len(list))
	}
	trend := expect(t, alice.do("GET", fmt.Sprintf("/profiles/%d/trend?mode=ampm", grandmaID), nil), 200, "alice trend").body
	if n := len(trend["buckets"].([]any)); n != len(readingpkg.Periods)*defaultRangeDays {
		t.Fatalf("ampm buckets = %d, want %d", n, len(readingpkg.Periods)*defaultRangeDays)
	}

	// ...but cannot change anything of Grandma's.
	write := map[string]any{"items": []map[string]any{{"systolic": 120, "diastolic": 80, "pulse": 70}}}
	expect(t, alice.do("POST", fmt.Sprintf("/profiles/%d/readings", grandmaID), write), 403, "alice writes grandma reading")
	expect(t, alice.do("PUT", fmt.Sprintf("/readings/%d", readingID), write), 403, "alice edits grandma reading")
	expect(t, alice.do("DELETE", fmt.Sprintf("/readings/%d", readingID), nil), 403, "alice deletes grandma reading")
	expect(t, alice.do("PUT", fmt.Sprintf("/profiles/%d", grandmaID), map[string]any{"displayName": "X"}), 403, "alice edits grandma profile")
	expect(t, alice.do("DELETE", fmt.Sprintf("/profiles/%d", grandmaID), nil), 403, "alice deletes grandma profile")

	// Outsiders see nothing.
	expect(t, carol.do("GET", fmt.Sprintf("/profiles/%d/readings", grandmaID), nil), 403, "carol reads grandma")
	expect(t, carol.do("GET", fmt.Sprintf("/profiles/%d/trend", grandmaID), nil), 403, "carol trend")
	expect(t, carol.do("GET", fmt.Sprintf("/readings/%d", readingID), nil), 403, "carol reading detail")

	// Self profiles cannot be deleted, even by their owner.
	expect(t, alice.do("DELETE", fmt.Sprintf("/profiles/%d", aliceSelf), nil), 403, "delete self profile")

	// Bob may edit his managed reading.
	edited := expect(t, bob.do("PUT", fmt.Sprintf("/readings/%d", readingID), write), 200, "bob edits").body
	if num(edited["systolic"]) != 120 || len(edited["items"].([]any)) != 1 {
		t.Fatalf("edited reading = %v", edited)
	}

	// Validation errors are structured.
	bad := expect(t, alice.do("POST", fmt.Sprintf("/profiles/%d/readings", aliceSelf), map[string]any{
		"items": []map[string]any{{"systolic": 80, "diastolic": 90, "pulse": 70}},
	}), 400, "invalid reading")
	if e := bad.body["error"].(map[string]any); e["field"] != "items[0].systolic" || e["reason"] != "not_above_diastolic" {
		t.Fatalf("validation error = %v", e)
	}

	// Leaving the family revokes read access immediately; managed profiles follow their manager.
	expect(t, bob.do("POST", "/family/leave", nil), 204, "bob leaves")
	expect(t, alice.do("GET", fmt.Sprintf("/profiles/%d/readings", grandmaID), nil), 403, "alice after bob left")
	expect(t, bob.do("GET", fmt.Sprintf("/profiles/%d/readings", grandmaID), nil), 200, "bob keeps his managed profile")
	expect(t, bob.do("GET", fmt.Sprintf("/profiles/%d/readings", aliceSelf), nil), 403, "bob after leaving")

	// Bob deletes Grandma, which removes her readings too.
	expect(t, bob.do("DELETE", fmt.Sprintf("/profiles/%d", grandmaID), nil), 204, "bob deletes grandma")
	expect(t, bob.do("GET", fmt.Sprintf("/readings/%d", readingID), nil), 404, "reading gone with profile")

	expect(t, alice.do("POST", "/auth/logout", nil), 204, "logout")
	expect(t, alice.do("GET", "/me", nil), 401, "me after logout")
}

func TestCSRFGuardAndLoginThrottle(t *testing.T) {
	base := newTestServer(t)
	c := newClient(t, base)

	r := c.request("POST", "/auth/register", map[string]any{
		"username": "alice", "password": "secret1", "displayName": "Alice",
	}, false)
	if r.status != 403 || r.errCode() != "csrf" {
		t.Fatalf("missing CSRF header: status=%d code=%s", r.status, r.errCode())
	}
	expect(t, c.do("POST", "/auth/register", map[string]any{
		"username": "alice", "password": "secret1", "displayName": "Alice",
	}), 201, "register with header")

	attacker := newClient(t, base)
	for i := 0; i < 5; i++ {
		res := expect(t, attacker.do("POST", "/auth/login", map[string]any{"username": "alice", "password": "nope"}), 400, "bad login")
		if res.errCode() != "invalid_credentials" {
			t.Fatalf("bad login code = %s", res.errCode())
		}
	}
	locked := expect(t, attacker.do("POST", "/auth/login", map[string]any{"username": "alice", "password": "secret1"}), 429, "locked out")
	if locked.errCode() != "too_many_attempts" {
		t.Fatalf("lockout code = %s", locked.errCode())
	}
}

func TestPasswordChangeSignsOutOtherDevices(t *testing.T) {
	base := newTestServer(t)
	phone, tablet := newClient(t, base), newClient(t, base)

	expect(t, phone.do("POST", "/auth/register", map[string]any{
		"username": "alice", "password": "secret1", "displayName": "Alice",
	}), 201, "register")
	expect(t, tablet.do("POST", "/auth/login", map[string]any{"username": "alice", "password": "secret1"}), 200, "second device")

	expect(t, phone.do("PUT", "/me/password", map[string]any{"oldPassword": "wrong", "newPassword": "secret2"}), 400, "wrong old password")
	expect(t, phone.do("PUT", "/me/password", map[string]any{"oldPassword": "secret1", "newPassword": "secret2"}), 204, "change password")

	expect(t, phone.do("GET", "/me", nil), 200, "current device stays signed in")
	expect(t, tablet.do("GET", "/me", nil), 401, "other device signed out")
	expect(t, newClient(t, base).do("POST", "/auth/login", map[string]any{"username": "alice", "password": "secret2"}), 200, "login with new password")
}

func TestPasswordChangeRollsBackWhenSessionRevocationFails(t *testing.T) {
	base, d := newTestServerWithDB(t)
	phone, tablet := newClient(t, base), newClient(t, base)
	expect(t, phone.do("POST", "/auth/register", map[string]any{
		"username": "alice", "password": "secret1", "displayName": "Alice",
	}), 201, "register")
	expect(t, tablet.do("POST", "/auth/login", map[string]any{
		"username": "alice", "password": "secret1",
	}), 200, "sign in on tablet")
	if _, err := d.Exec(`CREATE TRIGGER fail_revocation BEFORE DELETE ON sessions BEGIN
		SELECT RAISE(ABORT, 'session storage failure'); END`); err != nil {
		t.Fatal(err)
	}

	expect(t, phone.do("PUT", "/me/password", map[string]any{
		"oldPassword": "secret1", "newPassword": "secret2",
	}), 500, "revocation fails")
	expect(t, tablet.do("GET", "/me", nil), 200, "tablet session still valid")
	expect(t, newClient(t, base).do("POST", "/auth/login", map[string]any{
		"username": "alice", "password": "secret1",
	}), 200, "old password still valid")
}

func TestReadingPaginationIncludesOlderRecords(t *testing.T) {
	base, d := newTestServerWithDB(t)
	c := newClient(t, base)
	me := expect(t, c.do("POST", "/auth/register", map[string]any{
		"username": "alice", "password": "secret1", "displayName": "Alice",
	}), 201, "register")
	pid := num(me.body["selfProfileId"])
	userID := num(me.body["id"])
	path := fmt.Sprintf("/profiles/%d/readings", pid)
	at := time.Now().AddDate(0, 0, -60).Truncate(time.Second)
	if _, err := d.Exec(`
		WITH RECURSIVE series(n) AS (VALUES (1) UNION ALL SELECT n + 1 FROM series WHERE n < 1002)
		INSERT INTO readings (profile_id, measured_at, systolic, diastolic, created_by, created_at)
		SELECT ?, ?, 120, 80, ?, ? FROM series`, pid, at.Unix(), userID, at.Unix()); err != nil {
		t.Fatal(err)
	}
	created := expect(t, c.do("POST", path, map[string]any{
		"measuredAt": at, "items": []map[string]any{{"systolic": 125, "diastolic": 82}},
	}), 201, "save reading without pulse")
	id := num(created.body["id"])
	if created.body["pulse"] != nil {
		t.Fatalf("omitted pulse = %v", created.body["pulse"])
	}

	latest := expect(t, c.do("GET", path+"?limit=1", nil), 200, "latest across dates")
	if len(latest.list) != 1 || num(latest.list[0].(map[string]any)["id"]) != id ||
		len(latest.list[0].(map[string]any)["items"].([]any)) != 1 {
		t.Fatalf("latest list = %v", latest.list)
	}
	page := expect(t, c.do("GET", path+"?limit=30&offset=1000", nil), 200, "page beyond 1000")
	if len(page.list) != 3 || num(page.list[0].(map[string]any)["id"]) != id-1000 {
		t.Fatalf("page = %v", page.list)
	}
	after := at.AddDate(0, 0, 2).Format("2006-01-02")
	filtered := expect(t, c.do("GET", path+"?from="+after+"&limit=1", nil), 200, "respect date filter")
	if len(filtered.list) != 0 {
		t.Fatalf("dated list = %v", filtered.list)
	}
}
