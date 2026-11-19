package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"bptracker/internal/db"
)

const (
	// SessionTTL is long on purpose: elderly users should rarely face the login screen.
	SessionTTL = 90 * 24 * time.Hour
	// Sessions with less than this remaining are extended on use (sliding expiry).
	refreshBelow = 60 * 24 * time.Hour
)

var ErrNoSession = errors.New("no valid session")

type Session struct {
	Token     string
	ExpiresAt time.Time
}

type Sessions struct {
	DB  *sql.DB
	Now func() time.Time
}

func (s *Sessions) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// Create issues a new random token. Only its SHA-256 is stored, so a leaked database
// does not expose usable session tokens.
func (s *Sessions) Create(ctx context.Context, u User) (Session, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return Session{}, fmt.Errorf("generate token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	now := s.now()
	expires := now.Add(SessionTTL)

	if _, err := s.DB.ExecContext(ctx,
		`DELETE FROM sessions WHERE user_id = ? AND expires_at <= ?`, u.ID, now.Unix()); err != nil {
		return Session{}, fmt.Errorf("prune sessions: %w", err)
	}
	// A login verified before a concurrent password change must not issue a new session afterward.
	result, err := s.DB.ExecContext(ctx, `
		INSERT INTO sessions (token_hash, user_id, expires_at, created_at)
		SELECT ?, id, ?, ? FROM users WHERE id = ? AND password_hash = ?`,
		hashToken(token), expires.Unix(), now.Unix(), u.ID, u.passwordHash)
	if err != nil {
		return Session{}, fmt.Errorf("insert session: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return Session{}, fmt.Errorf("insert session rows: %w", err)
	}
	if n == 0 {
		return Session{}, ErrInvalidCredentials
	}
	return Session{Token: token, ExpiresAt: expires}, nil
}

// Lookup resolves a token to its user. When the session is extended, the returned
// Session carries the new expiry and refreshed is true so the caller can reissue the cookie.
func (s *Sessions) Lookup(ctx context.Context, token string) (u User, sess Session, refreshed bool, err error) {
	if token == "" {
		return User{}, Session{}, false, ErrNoSession
	}
	var (
		familyID  sql.NullInt64
		expiresAt int64
	)
	h := hashToken(token)
	err = s.DB.QueryRowContext(ctx, `
		SELECT u.id, u.username, u.family_id, s.expires_at
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = ?`, h).
		Scan(&u.ID, &u.Username, &familyID, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, Session{}, false, ErrNoSession
	}
	if err != nil {
		return User{}, Session{}, false, fmt.Errorf("lookup session: %w", err)
	}
	u.FamilyID = familyID.Int64

	now := s.now()
	expires := time.Unix(expiresAt, 0)
	if !expires.After(now) {
		return User{}, Session{}, false, ErrNoSession
	}
	if expires.Sub(now) < refreshBelow {
		expires = now.Add(SessionTTL)
		if _, err := s.DB.ExecContext(ctx,
			`UPDATE sessions SET expires_at = ? WHERE token_hash = ?`, expires.Unix(), h); err != nil {
			return User{}, Session{}, false, fmt.Errorf("extend session: %w", err)
		}
		refreshed = true
	}
	return u, Session{Token: token, ExpiresAt: expires}, refreshed, nil
}

func (s *Sessions) Delete(ctx context.Context, token string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, hashToken(token))
	return err
}

// DeleteOtherSessions signs the user out everywhere except the current device.
func DeleteOtherSessions(ctx context.Context, q db.Querier, userID int64, keepToken string) error {
	_, err := q.ExecContext(ctx,
		`DELETE FROM sessions WHERE user_id = ? AND token_hash <> ?`, userID, hashToken(keepToken))
	return err
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
