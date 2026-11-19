// Package ocr runs the bp-ocr command-line reader for one Omron HEM-1026 photo.
// The photo is written to a temporary file and removed before Read returns.
package ocr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

const DefaultTimeout = 20 * time.Second

var (
	ErrUnavailable = errors.New("ocr unavailable")
	ErrFailed      = errors.New("ocr failed")
)

// Result is the three readings. A nil field was not recognized.
// The high-indicator flag and per-digit confidence stay inside bp-ocr.
type Result struct {
	Systolic  *int `json:"systolic"`
	Diastolic *int `json:"diastolic"`
	Pulse     *int `json:"pulse"`
}

// Read runs `python -m bp_ocr <tempfile>` with dir as the working directory.
// Exit code 0 or 1 with JSON is a completed read (1 means at least one field is null).
// Any other exit, a timeout, or unparseable output is ErrFailed.
func Read(ctx context.Context, python, dir string, photo []byte) (Result, error) {
	if python == "" || dir == "" {
		return Result{}, ErrUnavailable
	}
	f, err := os.CreateTemp("", "bp-ocr-*.jpg")
	if err != nil {
		return Result{}, ErrFailed
	}
	path := f.Name()
	defer os.Remove(path)
	if _, err := f.Write(photo); err != nil {
		f.Close()
		return Result{}, ErrFailed
	}
	if err := f.Close(); err != nil {
		return Result{}, ErrFailed
	}

	cmd := exec.CommandContext(ctx, python, "-m", "bp_ocr", path)
	cmd.Dir = dir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	// Drop stderr so a photo path in a bp-ocr diagnostic cannot reach the server log.
	cmd.Stderr = io.Discard

	runErr := cmd.Run()
	if ctx.Err() != nil {
		return Result{}, ErrFailed
	}
	if runErr != nil {
		var exitErr *exec.ExitError
		if !errors.As(runErr, &exitErr) || exitErr.ExitCode() != 1 {
			return Result{}, ErrFailed
		}
	}
	var result Result
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrFailed, err)
	}
	return result, nil
}
