package httpapi

import (
	"errors"
	"net/http"
	"time"

	"bptracker/internal/auth"
)

const sessionCookie = "bp_session"

// authed resolves the session cookie and passes the user to fn, or answers 401.
func (s *Server) authed(fn func(http.ResponseWriter, *http.Request, auth.User) error) http.HandlerFunc {
	return s.handle(func(w http.ResponseWriter, r *http.Request) error {
		c, err := r.Cookie(sessionCookie)
		if err != nil {
			return errUnauthorized
		}
		u, sess, refreshed, err := s.sessions.Lookup(r.Context(), c.Value)
		if errors.Is(err, auth.ErrNoSession) {
			s.clearSessionCookie(w)
			return errUnauthorized
		}
		if err != nil {
			return err
		}
		if refreshed {
			s.setSessionCookie(w, sess)
		}
		return fn(w, r, u)
	})
}

func (s *Server) setSessionCookie(w http.ResponseWriter, sess auth.Session) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    sess.Token,
		Path:     "/",
		Expires:  sess.ExpiresAt,
		MaxAge:   int(sess.ExpiresAt.Sub(s.now()) / time.Second),
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}
