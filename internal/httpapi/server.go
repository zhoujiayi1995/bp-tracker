// Package httpapi wires the REST API and the embedded frontend into one http.Handler.
package httpapi

import (
	"database/sql"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"bptracker/internal/auth"
)

type Options struct {
	DB           *sql.DB
	Location     *time.Location
	CookieSecure bool
	Static       fs.FS // nil disables frontend serving (tests)
	Logger       *slog.Logger
	Now          func() time.Time
}

type Server struct {
	db           *sql.DB
	loc          *time.Location
	cookieSecure bool
	static       fs.FS
	log          *slog.Logger
	now          func() time.Time

	sessions     *auth.Sessions
	loginLimiter *auth.Limiter
	joinLimiter  *auth.Limiter
}

func New(o Options) *Server {
	now := o.Now
	if now == nil {
		now = time.Now
	}
	logger := o.Logger
	if logger == nil {
		logger = slog.Default()
	}
	loc := o.Location
	if loc == nil {
		loc = time.Local
	}
	return &Server{
		db:           o.DB,
		loc:          loc,
		cookieSecure: o.CookieSecure,
		static:       o.Static,
		log:          logger,
		now:          now,
		sessions:     &auth.Sessions{DB: o.DB, Now: now},
		loginLimiter: &auth.Limiter{MaxFailures: 5, Window: 15 * time.Minute, Lockout: 15 * time.Minute, Now: now},
		// Invite codes are only 6 digits, so guessing must be throttled hard.
		joinLimiter: &auth.Limiter{MaxFailures: 5, Window: time.Hour, Lockout: time.Hour, Now: now},
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/auth/register", s.handle(s.register))
	mux.HandleFunc("POST /api/auth/login", s.handle(s.login))
	mux.HandleFunc("POST /api/auth/logout", s.handle(s.logout))
	mux.HandleFunc("GET /api/me", s.authed(s.getMe))
	mux.HandleFunc("PUT /api/me/password", s.authed(s.changePassword))

	mux.HandleFunc("GET /api/family", s.authed(s.getFamily))
	mux.HandleFunc("POST /api/family", s.authed(s.createFamily))
	mux.HandleFunc("POST /api/family/join", s.authed(s.joinFamily))
	mux.HandleFunc("POST /api/family/leave", s.authed(s.leaveFamily))
	mux.HandleFunc("POST /api/family/invite-code/reset", s.authed(s.resetInviteCode))

	mux.HandleFunc("GET /api/profiles", s.authed(s.listProfiles))
	mux.HandleFunc("POST /api/profiles", s.authed(s.createProfile))
	mux.HandleFunc("PUT /api/profiles/{id}", s.authed(s.updateProfile))
	mux.HandleFunc("DELETE /api/profiles/{id}", s.authed(s.deleteProfile))

	mux.HandleFunc("POST /api/profiles/{id}/readings", s.authed(s.createReading))
	mux.HandleFunc("GET /api/profiles/{id}/readings", s.authed(s.listReadings))
	mux.HandleFunc("GET /api/profiles/{id}/trend", s.authed(s.trend))
	mux.HandleFunc("GET /api/readings/{id}", s.authed(s.getReading))
	mux.HandleFunc("PUT /api/readings/{id}", s.authed(s.updateReading))
	mux.HandleFunc("DELETE /api/readings/{id}", s.authed(s.deleteReading))

	mux.HandleFunc("/api/", s.handle(func(http.ResponseWriter, *http.Request) error {
		return errNotFound
	}))
	if s.static != nil {
		mux.Handle("/", spaHandler(s.static))
	}

	return s.logRequests(securityHeaders(csrfGuard(mux)))
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

// csrfGuard requires a custom header on state-changing API calls. Browsers cannot attach
// custom headers cross-site without a CORS preflight, which this server never approves;
// combined with SameSite=Lax cookies this blocks cross-site request forgery.
func csrfGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") && r.Method != http.MethodGet && r.Method != http.MethodHead {
			if r.Header.Get("X-Requested-With") == "" {
				writeJSON(w, http.StatusForbidden, errorBody{Error: errorDetail{Code: "csrf"}})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.log.Info("api", "method", r.Method, "path", r.URL.Path, "status", rec.status,
			"duration_ms", time.Since(start).Milliseconds())
	})
}
