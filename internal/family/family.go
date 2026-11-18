// Package family groups accounts so members can view each other's data read-only.
// Each account belongs to at most one family. Functions that read and then write expect
// to run inside a transaction.
package family

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
	"unicode/utf8"

	"bptracker/internal/db"
	"bptracker/internal/validate"
)

var (
	ErrAlreadyInFamily   = errors.New("already in a family")
	ErrNotInFamily       = errors.New("not in a family")
	ErrInvalidInviteCode = errors.New("invalid invite code")
)

const (
	inviteCodeLen  = 6
	maxNameRunes   = 20
	maxCodeRetries = 8
)

type Family struct {
	ID         int64
	Name       string
	InviteCode string
	CreatedBy  int64
}

type Member struct {
	UserID      int64
	Username    string
	DisplayName string
}

func Create(ctx context.Context, q db.Querier, userID int64, name string, now time.Time) (Family, error) {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n == 0 || n > maxNameRunes {
		return Family{}, validate.Field("name", "length")
	}
	current, err := familyOf(ctx, q, userID)
	if err != nil {
		return Family{}, err
	}
	if current != 0 {
		return Family{}, ErrAlreadyInFamily
	}

	f := Family{Name: name, CreatedBy: userID}
	for attempt := 0; ; attempt++ {
		if f.InviteCode, err = newInviteCode(); err != nil {
			return Family{}, err
		}
		res, err := q.ExecContext(ctx,
			`INSERT INTO families (name, invite_code, created_by, created_at) VALUES (?, ?, ?, ?)`,
			f.Name, f.InviteCode, userID, now.Unix())
		if db.IsUniqueViolation(err) && attempt < maxCodeRetries {
			continue
		}
		if err != nil {
			return Family{}, fmt.Errorf("insert family: %w", err)
		}
		if f.ID, err = res.LastInsertId(); err != nil {
			return Family{}, err
		}
		break
	}

	if _, err := q.ExecContext(ctx, `UPDATE users SET family_id = ? WHERE id = ?`, f.ID, userID); err != nil {
		return Family{}, fmt.Errorf("assign family: %w", err)
	}
	return f, nil
}

func Join(ctx context.Context, q db.Querier, userID int64, code string) (Family, error) {
	code = strings.TrimSpace(code)
	if len(code) != inviteCodeLen {
		return Family{}, ErrInvalidInviteCode
	}
	current, err := familyOf(ctx, q, userID)
	if err != nil {
		return Family{}, err
	}
	if current != 0 {
		return Family{}, ErrAlreadyInFamily
	}

	var f Family
	err = q.QueryRowContext(ctx,
		`SELECT id, name, invite_code, created_by FROM families WHERE invite_code = ?`, code).
		Scan(&f.ID, &f.Name, &f.InviteCode, &f.CreatedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return Family{}, ErrInvalidInviteCode
	}
	if err != nil {
		return Family{}, fmt.Errorf("find family: %w", err)
	}
	if _, err := q.ExecContext(ctx, `UPDATE users SET family_id = ? WHERE id = ?`, f.ID, userID); err != nil {
		return Family{}, fmt.Errorf("join family: %w", err)
	}
	return f, nil
}

// Leave removes the user from their family and deletes the family once it is empty.
// Managed profiles follow their manager, so they leave too.
func Leave(ctx context.Context, q db.Querier, userID int64) error {
	familyID, err := familyOf(ctx, q, userID)
	if err != nil {
		return err
	}
	if familyID == 0 {
		return ErrNotInFamily
	}
	if _, err := q.ExecContext(ctx, `UPDATE users SET family_id = NULL WHERE id = ?`, userID); err != nil {
		return fmt.Errorf("leave family: %w", err)
	}
	if _, err := q.ExecContext(ctx, `
		DELETE FROM families WHERE id = ?
		AND NOT EXISTS (SELECT 1 FROM users WHERE family_id = ?)`, familyID, familyID); err != nil {
		return fmt.Errorf("delete empty family: %w", err)
	}
	return nil
}

// ResetInviteCode invalidates the old code, e.g. after it was shared too widely.
func ResetInviteCode(ctx context.Context, q db.Querier, userID int64) (string, error) {
	familyID, err := familyOf(ctx, q, userID)
	if err != nil {
		return "", err
	}
	if familyID == 0 {
		return "", ErrNotInFamily
	}
	for attempt := 0; ; attempt++ {
		code, err := newInviteCode()
		if err != nil {
			return "", err
		}
		_, err = q.ExecContext(ctx, `UPDATE families SET invite_code = ? WHERE id = ?`, code, familyID)
		if db.IsUniqueViolation(err) && attempt < maxCodeRetries {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("reset invite code: %w", err)
		}
		return code, nil
	}
}

func Get(ctx context.Context, q db.Querier, familyID int64) (Family, error) {
	var f Family
	err := q.QueryRowContext(ctx,
		`SELECT id, name, invite_code, created_by FROM families WHERE id = ?`, familyID).
		Scan(&f.ID, &f.Name, &f.InviteCode, &f.CreatedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return Family{}, ErrNotInFamily
	}
	if err != nil {
		return Family{}, fmt.Errorf("get family: %w", err)
	}
	return f, nil
}

func Members(ctx context.Context, q db.Querier, familyID int64) ([]Member, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT u.id, u.username, COALESCE(p.display_name, u.username)
		FROM users u LEFT JOIN profiles p ON p.user_id = u.id
		WHERE u.family_id = ?
		ORDER BY u.id`, familyID)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	defer rows.Close()
	var out []Member
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.UserID, &m.Username, &m.DisplayName); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func familyOf(ctx context.Context, q db.Querier, userID int64) (int64, error) {
	var id sql.NullInt64
	if err := q.QueryRowContext(ctx, `SELECT family_id FROM users WHERE id = ?`, userID).Scan(&id); err != nil {
		return 0, fmt.Errorf("load user family: %w", err)
	}
	return id.Int64, nil
}

func newInviteCode() (string, error) {
	max := big.NewInt(1_000_000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", fmt.Errorf("generate invite code: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}
