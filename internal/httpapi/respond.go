package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"bptracker/internal/auth"
	"bptracker/internal/family"
	"bptracker/internal/profile"
	"bptracker/internal/reading"
	"bptracker/internal/validate"
)

const maxBodyBytes = 64 << 10

// apiError is an error with an explicit HTTP status and a stable machine-readable code.
// User-facing wording lives in the frontend, keyed by code.
type apiError struct {
	status     int
	code       string
	retryAfter int // seconds; only for 429
}

func (e *apiError) Error() string { return e.code }

var (
	errUnauthorized = &apiError{status: http.StatusUnauthorized, code: "unauthorized"}
	errForbidden    = &apiError{status: http.StatusForbidden, code: "forbidden"}
	errNotFound     = &apiError{status: http.StatusNotFound, code: "not_found"}
	errBadJSON      = &apiError{status: http.StatusBadRequest, code: "bad_json"}
)

type errorDetail struct {
	Code       string `json:"code"`
	Field      string `json:"field,omitempty"`
	Reason     string `json:"reason,omitempty"`
	RetryAfter int    `json:"retryAfter,omitempty"`
}

type errorBody struct {
	Error errorDetail `json:"error"`
}

func (s *Server) handle(fn func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := fn(w, r); err != nil {
			s.writeError(w, r, err)
		}
	}
}

func (s *Server) writeError(w http.ResponseWriter, r *http.Request, err error) {
	var (
		ae *apiError
		ve *validate.Error
	)
	switch {
	case errors.As(err, &ae):
		if ae.retryAfter > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(ae.retryAfter))
		}
		writeJSON(w, ae.status, errorBody{Error: errorDetail{Code: ae.code, RetryAfter: ae.retryAfter}})
	case errors.As(err, &ve):
		writeJSON(w, http.StatusBadRequest, errorBody{Error: errorDetail{Code: "invalid_input", Field: ve.Field, Reason: ve.Reason}})
	case errors.Is(err, auth.ErrUsernameTaken):
		writeCode(w, http.StatusConflict, "username_taken")
	case errors.Is(err, auth.ErrInvalidCredentials):
		// Not 401: the client reserves 401 for "session expired, go to login".
		writeCode(w, http.StatusBadRequest, "invalid_credentials")
	case errors.Is(err, family.ErrAlreadyInFamily):
		writeCode(w, http.StatusConflict, "already_in_family")
	case errors.Is(err, family.ErrNotInFamily):
		writeCode(w, http.StatusConflict, "not_in_family")
	case errors.Is(err, family.ErrInvalidInviteCode):
		writeCode(w, http.StatusBadRequest, "invalid_invite_code")
	case errors.Is(err, profile.ErrNotFound), errors.Is(err, reading.ErrNotFound):
		writeCode(w, http.StatusNotFound, "not_found")
	default:
		s.log.Error("request failed", "method", r.Method, "path", r.URL.Path, "err", err)
		writeCode(w, http.StatusInternalServerError, "internal")
	}
}

func writeCode(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, errorBody{Error: errorDetail{Code: code}})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		return fmt.Errorf("%w: %v", errBadJSON, err)
	}
	return nil
}

func pathID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, errNotFound
	}
	return id, nil
}
