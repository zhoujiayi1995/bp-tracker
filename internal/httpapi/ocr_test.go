package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"bptracker/internal/db"
)

func newOCRServer(t *testing.T, python, dir string) string {
	t.Helper()
	d, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "bp.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	loc, _ := time.LoadLocation("Asia/Shanghai")
	srv := New(Options{
		DB:        d,
		Location:  loc,
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		OCRPython: python,
		OCRDir:    dir,
	})
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts.URL
}

func (c *client) postPhoto(path string, content []byte) response {
	c.t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("photo", "screen.jpg")
	if err != nil {
		c.t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		c.t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		c.t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, c.base+"/api"+path, &buf)
	if err != nil {
		c.t.Fatal(err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("X-Requested-With", "test")
	res, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	out := response{status: res.StatusCode}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out.body)
	}
	return out
}

func TestOCRRequiresLogin(t *testing.T) {
	base := newOCRServer(t, "/bin/echo", t.TempDir())
	got := newClient(t, base).postPhoto("/ocr", []byte("photo"))
	expect(t, got, http.StatusUnauthorized, "anonymous ocr")
}

func TestOCRUnavailableWhenUnconfigured(t *testing.T) {
	base := newOCRServer(t, "", "")
	c := newClient(t, base)
	expect(t, c.do("POST", "/auth/register", map[string]any{
		"username": "ocr-off", "password": "secret1", "displayName": "Off",
	}), 201, "register")
	got := expect(t, c.postPhoto("/ocr", []byte("photo")), http.StatusServiceUnavailable, "unconfigured")
	if got.errCode() != "ocr_unavailable" {
		t.Fatalf("code = %s", got.errCode())
	}
}

func TestOCRResults(t *testing.T) {
	dir := t.TempDir()
	python := writeOCRScript(t, `
case "$OCR_MODE" in
  ok)
    printf '%s\n' '{"systolic":118,"diastolic":63,"pulse":72,"high_indicator":true,"confidence":{"systolic":1}}'
    exit 0
    ;;
  partial)
    printf '%s\n' '{"systolic":118,"diastolic":null,"pulse":null}'
    exit 1
    ;;
  empty)
    printf '%s\n' '{"systolic":null,"diastolic":null,"pulse":null}'
    exit 1
    ;;
  fail)
    exit 2
    ;;
esac
exit 2
`)
	base := newOCRServer(t, python, dir)
	c := newClient(t, base)
	expect(t, c.do("POST", "/auth/register", map[string]any{
		"username": "ocr-user", "password": "secret1", "displayName": "User",
	}), 201, "register")

	t.Setenv("OCR_MODE", "ok")
	ok := expect(t, c.postPhoto("/ocr", []byte("photo")), http.StatusOK, "exit 0")
	if ok.body["systolic"].(float64) != 118 || ok.body["diastolic"].(float64) != 63 || ok.body["pulse"].(float64) != 72 {
		t.Fatalf("body = %v", ok.body)
	}
	if _, extra := ok.body["high_indicator"]; extra {
		t.Fatalf("high_indicator leaked: %v", ok.body)
	}

	t.Setenv("OCR_MODE", "partial")
	partial := expect(t, c.postPhoto("/ocr", []byte("photo")), http.StatusOK, "exit 1 partial")
	if partial.body["systolic"].(float64) != 118 || partial.body["diastolic"] != nil || partial.body["pulse"] != nil {
		t.Fatalf("body = %v", partial.body)
	}

	t.Setenv("OCR_MODE", "empty")
	empty := expect(t, c.postPhoto("/ocr", []byte("photo")), http.StatusOK, "all null")
	if empty.body["systolic"] != nil || empty.body["diastolic"] != nil || empty.body["pulse"] != nil {
		t.Fatalf("body = %v", empty.body)
	}

	t.Setenv("OCR_MODE", "fail")
	failed := expect(t, c.postPhoto("/ocr", []byte("photo")), http.StatusUnprocessableEntity, "exit 2")
	if failed.errCode() != "ocr_failed" {
		t.Fatalf("code = %s", failed.errCode())
	}
}

func TestOCRBusy(t *testing.T) {
	dir := t.TempDir()
	hold := filepath.Join(dir, "hold")
	release := filepath.Join(dir, "release")
	t.Setenv("OCR_HOLD", hold)
	t.Setenv("OCR_RELEASE", release)
	python := writeOCRScript(t, `
printf 'started\n' >> "$OCR_HOLD"
i=0
while [ ! -f "$OCR_RELEASE" ]; do
  i=$((i + 1))
  if [ "$i" -gt 100 ]; then
    exit 2
  fi
  sleep 0.05
done
printf '%s\n' '{"systolic":120,"diastolic":80,"pulse":60}'
exit 0
`)
	base := newOCRServer(t, python, dir)
	c := newClient(t, base)
	expect(t, c.do("POST", "/auth/register", map[string]any{
		"username": "ocr-busy", "password": "secret1", "displayName": "Busy",
	}), 201, "register")

	first := make(chan response, 1)
	go func() { first <- c.postPhoto("/ocr", []byte("photo")) }()
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(hold); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("ocr did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	busy := expect(t, c.postPhoto("/ocr", []byte("other")), http.StatusTooManyRequests, "busy")
	if busy.errCode() != "ocr_busy" {
		t.Fatalf("code = %s", busy.errCode())
	}
	if err := os.WriteFile(release, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := expect(t, <-first, http.StatusOK, "first finishes")
	if got.body["systolic"].(float64) != 120 {
		t.Fatalf("body = %v", got.body)
	}
}

func writeOCRScript(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "python")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
