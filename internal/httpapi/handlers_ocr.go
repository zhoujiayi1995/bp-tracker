package httpapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"bptracker/internal/auth"
	"bptracker/internal/ocr"
)

const maxOCRBytes = 8 << 20

var (
	errOCRUnavailable = &apiError{status: http.StatusServiceUnavailable, code: "ocr_unavailable"}
	errOCRBusy        = &apiError{status: http.StatusTooManyRequests, code: "ocr_busy"}
	errOCRFailed      = &apiError{status: http.StatusUnprocessableEntity, code: "ocr_failed"}
)

func (s *Server) recognizePhoto(w http.ResponseWriter, r *http.Request, _ auth.User) error {
	if s.ocrPython == "" || s.ocrDir == "" {
		return errOCRUnavailable
	}
	photo, err := readPhotoUpload(w, r)
	if err != nil {
		return errOCRFailed
	}
	select {
	case s.ocrSlot <- struct{}{}:
		defer func() { <-s.ocrSlot }()
	default:
		return errOCRBusy
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.ocrTimeout)
	defer cancel()

	started := time.Now()
	result, err := ocr.Read(ctx, s.ocrPython, s.ocrDir, photo)
	// Duration and outcome only. Readings and the photo stay out of the log.
	s.log.Info("ocr", "ok", err == nil, "duration_ms", time.Since(started).Milliseconds())
	if err != nil {
		if errors.Is(err, ocr.ErrUnavailable) {
			return errOCRUnavailable
		}
		return errOCRFailed
	}
	writeJSON(w, http.StatusOK, result)
	return nil
}

func readPhotoUpload(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxOCRBytes)
	if err := r.ParseMultipartForm(maxOCRBytes); err != nil {
		return nil, err
	}
	defer func() {
		if r.MultipartForm != nil {
			r.MultipartForm.RemoveAll()
		}
	}()
	file, _, err := r.FormFile("photo")
	if err != nil {
		return nil, err
	}
	defer file.Close()
	photo, err := io.ReadAll(io.LimitReader(file, maxOCRBytes+1))
	if err != nil || len(photo) == 0 || len(photo) > maxOCRBytes {
		if err == nil {
			err = errOCRFailed
		}
		return nil, err
	}
	return photo, nil
}
