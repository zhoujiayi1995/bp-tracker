package ocr

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadRequiresInterpreterAndDirectory(t *testing.T) {
	if _, err := Read(context.Background(), "", t.TempDir(), []byte("x")); err != ErrUnavailable {
		t.Fatalf("empty python: %v", err)
	}
	if _, err := Read(context.Background(), "python", "", []byte("x")); err != ErrUnavailable {
		t.Fatalf("empty dir: %v", err)
	}
}

func TestReadExitCodesAndDeletesPhoto(t *testing.T) {
	dir := t.TempDir()
	seen := filepath.Join(dir, "seen")
	t.Setenv("OCR_SEEN", seen)
	python := writeScript(t, `
case "$1" in -m) ;; *) exit 2 ;; esac
[ "$2" = "bp_ocr" ] || exit 2
base=$(basename "$3")
case "$base" in bp-ocr-*.jpg) ;; *) exit 2 ;; esac
[ -f "$3" ] || exit 2
printf '%s' "$3" > "$OCR_SEEN"
case "$OCR_MODE" in
  ok)
    printf '%s\n' '{"systolic":118,"diastolic":63,"pulse":72,"high_indicator":true,"confidence":{"systolic":0.9}}'
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
  badjson)
    printf '%s\n' 'not-json'
    exit 0
    ;;
  fail)
    exit 2
    ;;
esac
exit 2
`)
	photo := []byte("photo-bytes")

	t.Setenv("OCR_MODE", "ok")
	os.Remove(seen)
	got, err := Read(context.Background(), python, dir, photo)
	if err != nil {
		t.Fatal(err)
	}
	if *got.Systolic != 118 || *got.Diastolic != 63 || *got.Pulse != 72 {
		t.Fatalf("result = %+v", got)
	}
	assertPhotoRemoved(t, seen)

	t.Setenv("OCR_MODE", "partial")
	got, err = Read(context.Background(), python, dir, photo)
	if err != nil {
		t.Fatal(err)
	}
	if got.Systolic == nil || *got.Systolic != 118 || got.Diastolic != nil || got.Pulse != nil {
		t.Fatalf("result = %+v", got)
	}

	t.Setenv("OCR_MODE", "empty")
	got, err = Read(context.Background(), python, dir, photo)
	if err != nil {
		t.Fatal(err)
	}
	if got.Systolic != nil || got.Diastolic != nil || got.Pulse != nil {
		t.Fatalf("result = %+v", got)
	}

	t.Setenv("OCR_MODE", "badjson")
	if _, err := Read(context.Background(), python, dir, photo); !errors.Is(err, ErrFailed) {
		t.Fatalf("err = %v", err)
	}

	t.Setenv("OCR_MODE", "fail")
	os.Remove(seen)
	if _, err := Read(context.Background(), python, dir, photo); !errors.Is(err, ErrFailed) {
		t.Fatalf("err = %v", err)
	}
	assertPhotoRemoved(t, seen)
}

func TestReadTimeoutDeletesPhoto(t *testing.T) {
	dir := t.TempDir()
	python := writeScript(t, `
printf '%s' "$3" > seen
exec sleep 30
`)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := Read(ctx, python, dir, []byte("photo-bytes")); !errors.Is(err, ErrFailed) {
		t.Fatalf("err = %v", err)
	}
	recorded, err := os.ReadFile(filepath.Join(dir, "seen"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(string(recorded)); !os.IsNotExist(err) {
		t.Fatalf("temp photo still present: %s (%v)", recorded, err)
	}
}

func assertPhotoRemoved(t *testing.T, seen string) {
	t.Helper()
	path, err := os.ReadFile(seen)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(string(path)); !os.IsNotExist(err) {
		t.Fatalf("temp photo still present: %s (%v)", path, err)
	}
}

func writeScript(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "python")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
