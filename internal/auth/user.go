// Package auth owns login accounts: credentials, sessions and login throttling.
package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"time"

	"golang.org/x/crypto/bcrypt"

	"bptracker/internal/authz"
	"bptracker/internal/db"
	"bptracker/internal/validate"
)

var (
	ErrUsernameTaken      = errors.New("username taken")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserNotFound       = errors.New("user not found")
)

const (
	minPasswordLen = 6
	maxPasswordLen = 72 // bcrypt ignores bytes beyond 72
)

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{3,32}$`)

var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("timing-equalizer"), bcrypt.DefaultCost)

type User struct {
	ID           int64
	Username     string
	FamilyID     int64 // 0 when the user has not joined a family
	passwordHash string
}

func (u User) Subject() authz.Subject {
	return authz.Subject{UserID: u.ID, FamilyID: u.FamilyID}
}

func ValidateUsername(username string) error {
	if !usernamePattern.MatchString(username) {
		return validate.Field("username", "format")
	}
	return nil
}

func ValidatePassword(password string) error {
	if len(password) < minPasswordLen || len(password) > maxPasswordLen {
		return validate.Field("password", "length")
	}
	return nil
}

func HashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(h), nil
}

// CreateUser validates input and inserts the account. It does not create the self profile;
// callers do that in the same transaction.
func CreateUser(ctx context.Context, q db.Querier, username, password string, now time.Time) (User, error) {
	if err := ValidateUsername(username); err != nil {
		return User{}, err
	}
	if err := ValidatePassword(password); err != nil {
		return User{}, err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return User{}, err
	}

	res, err := q.ExecContext(ctx,
		`INSERT INTO users (username, password_hash, created_at) VALUES (?, ?, ?)`,
		username, hash, now.Unix())
	if err != nil {
		if db.IsUniqueViolation(err) {
			return User{}, ErrUsernameTaken
		}
		return User{}, fmt.Errorf("insert user: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return User{}, fmt.Errorf("user id: %w", err)
	}
	return User{ID: id, Username: username, passwordHash: hash}, nil
}

// Authenticate checks the password. It returns ErrInvalidCredentials for both unknown
// usernames and wrong passwords so callers cannot leak which one failed.
func Authenticate(ctx context.Context, q db.Querier, username, password string) (User, error) {
	var (
		u        User
		familyID sql.NullInt64
	)
	err := q.QueryRowContext(ctx,
		`SELECT id, username, password_hash, family_id FROM users WHERE username = ?`, username).
		Scan(&u.ID, &u.Username, &u.passwordHash, &familyID)
	if errors.Is(err, sql.ErrNoRows) {
		// Spend the same bcrypt time as a real check so response timing does not reveal valid usernames.
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		return User{}, ErrInvalidCredentials
	}
	if err != nil {
		return User{}, fmt.Errorf("load user: %w", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(u.passwordHash), []byte(password)) != nil {
		return User{}, ErrInvalidCredentials
	}
	u.FamilyID = familyID.Int64
	return u, nil
}

func GetUser(ctx context.Context, q db.Querier, id int64) (User, error) {
	var (
		u        User
		familyID sql.NullInt64
	)
	err := q.QueryRowContext(ctx,
		`SELECT id, username, family_id FROM users WHERE id = ?`, id).
		Scan(&u.ID, &u.Username, &familyID)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("load user: %w", err)
	}
	u.FamilyID = familyID.Int64
	return u, nil
}

func ChangePassword(ctx context.Context, q db.Querier, userID int64, oldPassword, newPassword string) error {
	var hash string
	if err := q.QueryRowContext(ctx,
		`SELECT password_hash FROM users WHERE id = ?`, userID).Scan(&hash); err != nil {
		return fmt.Errorf("load password: %w", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(oldPassword)) != nil {
		return ErrInvalidCredentials
	}
	if err := ValidatePassword(newPassword); err != nil {
		return err
	}
	newHash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	if _, err := q.ExecContext(ctx,
		`UPDATE users SET password_hash = ? WHERE id = ?`, newHash, userID); err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	return nil
}
