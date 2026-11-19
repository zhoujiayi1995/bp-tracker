package httpapi

import (
	"database/sql"
	"errors"
	"math"
	"net"
	"net/http"
	"strings"

	"bptracker/internal/auth"
	"bptracker/internal/db"
	"bptracker/internal/profile"
)

type meDTO struct {
	ID            int64  `json:"id"`
	Username      string `json:"username"`
	DisplayName   string `json:"displayName"`
	SelfProfileID int64  `json:"selfProfileId"`
	FamilyID      *int64 `json:"familyId"`
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		DisplayName string `json:"displayName"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		return err
	}
	req.Username = strings.TrimSpace(req.Username)
	if err := profile.ValidateDisplayName(strings.TrimSpace(req.DisplayName)); err != nil {
		return err
	}

	var u auth.User
	err := db.WithTx(r.Context(), s.db, func(tx *sql.Tx) error {
		var err error
		now := s.now()
		if u, err = auth.CreateUser(r.Context(), tx, req.Username, req.Password, now); err != nil {
			return err
		}
		_, err = profile.CreateSelf(r.Context(), tx, u.ID, req.DisplayName, now)
		return err
	})
	if err != nil {
		return err
	}
	return s.startSession(w, r, u, http.StatusCreated)
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		return err
	}
	req.Username = strings.TrimSpace(req.Username)

	key := strings.ToLower(req.Username) + "|" + clientIP(r)
	if ok, wait := s.loginLimiter.Allow(key); !ok {
		return tooManyAttempts(wait.Seconds())
	}
	u, err := auth.Authenticate(r.Context(), s.db, req.Username, req.Password)
	if err != nil {
		s.loginLimiter.Fail(key)
		return err
	}
	s.loginLimiter.Reset(key)
	return s.startSession(w, r, u, http.StatusOK)
}

func (s *Server) startSession(w http.ResponseWriter, r *http.Request, u auth.User, status int) error {
	sess, err := s.sessions.Create(r.Context(), u)
	if err != nil {
		return err
	}
	s.setSessionCookie(w, sess)
	me, err := s.buildMe(r, u)
	if err != nil {
		return err
	}
	writeJSON(w, status, me)
	return nil
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) error {
	if c, err := r.Cookie(sessionCookie); err == nil {
		if err := s.sessions.Delete(r.Context(), c.Value); err != nil {
			return err
		}
	}
	s.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) getMe(w http.ResponseWriter, r *http.Request, u auth.User) error {
	me, err := s.buildMe(r, u)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, me)
	return nil
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request, u auth.User) error {
	var req struct {
		OldPassword string `json:"oldPassword"`
		NewPassword string `json:"newPassword"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		return err
	}
	key := "pw|" + u.Username
	if ok, wait := s.loginLimiter.Allow(key); !ok {
		return tooManyAttempts(wait.Seconds())
	}
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return errUnauthorized
	}
	ctx := r.Context()
	err = db.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		if err := auth.ChangePassword(ctx, tx, u.ID, req.OldPassword, req.NewPassword); err != nil {
			return err
		}
		return auth.DeleteOtherSessions(ctx, tx, u.ID, c.Value)
	})
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			s.loginLimiter.Fail(key)
		}
		return err
	}
	s.loginLimiter.Reset(key)

	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) buildMe(r *http.Request, u auth.User) (meDTO, error) {
	self, err := profile.GetSelf(r.Context(), s.db, u.ID)
	if err != nil {
		return meDTO{}, err
	}
	me := meDTO{ID: u.ID, Username: u.Username, DisplayName: self.DisplayName, SelfProfileID: self.ID}
	if u.FamilyID != 0 {
		fid := u.FamilyID
		me.FamilyID = &fid
	}
	return me, nil
}

func tooManyAttempts(seconds float64) error {
	return &apiError{status: http.StatusTooManyRequests, code: "too_many_attempts", retryAfter: int(math.Ceil(seconds))}
}

// clientIP deliberately ignores X-Forwarded-For: it is client-controlled unless a trusted
// proxy is configured, and throttling is keyed primarily on the username anyway.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
