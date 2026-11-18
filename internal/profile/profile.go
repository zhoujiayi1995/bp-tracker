// Package profile manages the people whose blood pressure is recorded. A profile either
// belongs to a login account (self profile) or is managed by one (for relatives who do not
// use the app themselves).
package profile

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"bptracker/internal/authz"
	"bptracker/internal/db"
	"bptracker/internal/validate"
)

var ErrNotFound = errors.New("profile not found")

const (
	maxNameRunes = 20
	maxTextRunes = 2000
	minBirthYear = 1900
)

type Profile struct {
	ID             int64
	DisplayName    string
	UserID         int64
	ManagerUserID  int64
	ManagerName    string
	Gender         string
	BirthYear      int
	MedicalHistory string
	Medications    string
	OwnerFamilyID  int64
	CreatedAt      time.Time
}

func (p Profile) IsManaged() bool { return p.ManagerUserID != 0 }

func (p Profile) Target() authz.Target {
	return authz.Target{UserID: p.UserID, ManagerUserID: p.ManagerUserID, OwnerFamilyID: p.OwnerFamilyID}
}

// Fields are the user-editable attributes of a profile.
type Fields struct {
	DisplayName    string
	Gender         string
	BirthYear      int
	MedicalHistory string
	Medications    string
}

// Normalize trims input in place and validates it.
func (f *Fields) Normalize(now time.Time) error {
	f.DisplayName = strings.TrimSpace(f.DisplayName)
	f.Gender = strings.TrimSpace(f.Gender)
	f.MedicalHistory = strings.TrimSpace(f.MedicalHistory)
	f.Medications = strings.TrimSpace(f.Medications)

	if err := ValidateDisplayName(f.DisplayName); err != nil {
		return err
	}
	switch f.Gender {
	case "", "male", "female":
	default:
		return validate.Field("gender", "unknown")
	}
	if f.BirthYear != 0 && (f.BirthYear < minBirthYear || f.BirthYear > now.Year()) {
		return validate.Field("birthYear", "range")
	}
	if utf8.RuneCountInString(f.MedicalHistory) > maxTextRunes {
		return validate.Field("medicalHistory", "too_long")
	}
	if utf8.RuneCountInString(f.Medications) > maxTextRunes {
		return validate.Field("medications", "too_long")
	}
	return nil
}

func ValidateDisplayName(name string) error {
	n := utf8.RuneCountInString(name)
	if n == 0 {
		return validate.Field("displayName", "required")
	}
	if n > maxNameRunes {
		return validate.Field("displayName", "too_long")
	}
	return nil
}

func CreateSelf(ctx context.Context, q db.Querier, userID int64, displayName string, now time.Time) (int64, error) {
	displayName = strings.TrimSpace(displayName)
	if err := ValidateDisplayName(displayName); err != nil {
		return 0, err
	}
	res, err := q.ExecContext(ctx,
		`INSERT INTO profiles (display_name, user_id, created_at) VALUES (?, ?, ?)`,
		displayName, userID, now.Unix())
	if err != nil {
		return 0, fmt.Errorf("insert self profile: %w", err)
	}
	return res.LastInsertId()
}

func CreateManaged(ctx context.Context, q db.Querier, managerID int64, f Fields, now time.Time) (int64, error) {
	if err := f.Normalize(now); err != nil {
		return 0, err
	}
	res, err := q.ExecContext(ctx, `
		INSERT INTO profiles (display_name, manager_user_id, gender, birth_year, medical_history, medications, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		f.DisplayName, managerID, f.Gender, nullInt(f.BirthYear), f.MedicalHistory, f.Medications, now.Unix())
	if err != nil {
		return 0, fmt.Errorf("insert managed profile: %w", err)
	}
	return res.LastInsertId()
}

func Update(ctx context.Context, q db.Querier, id int64, f Fields, now time.Time) error {
	if err := f.Normalize(now); err != nil {
		return err
	}
	res, err := q.ExecContext(ctx, `
		UPDATE profiles SET display_name = ?, gender = ?, birth_year = ?, medical_history = ?, medications = ?
		WHERE id = ?`,
		f.DisplayName, f.Gender, nullInt(f.BirthYear), f.MedicalHistory, f.Medications, id)
	if err != nil {
		return fmt.Errorf("update profile: %w", err)
	}
	return requireAffected(res)
}

// Delete removes a profile and, through ON DELETE CASCADE, all of its readings.
func Delete(ctx context.Context, q db.Querier, id int64) error {
	res, err := q.ExecContext(ctx, `DELETE FROM profiles WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete profile: %w", err)
	}
	return requireAffected(res)
}

// selectProfiles resolves the owning or managing account to derive the family that may
// read the profile, plus the manager's display name for managed profiles.
const selectProfiles = `
	SELECT p.id, p.display_name, COALESCE(p.user_id, 0), COALESCE(p.manager_user_id, 0),
	       COALESCE(mp.display_name, ''), p.gender, COALESCE(p.birth_year, 0),
	       p.medical_history, p.medications, COALESCE(u.family_id, 0), p.created_at
	FROM profiles p
	JOIN users u ON u.id = COALESCE(p.user_id, p.manager_user_id)
	LEFT JOIN profiles mp ON mp.user_id = p.manager_user_id`

func Get(ctx context.Context, q db.Querier, id int64) (Profile, error) {
	rows, err := q.QueryContext(ctx, selectProfiles+` WHERE p.id = ?`, id)
	if err != nil {
		return Profile{}, fmt.Errorf("get profile: %w", err)
	}
	list, err := scanProfiles(rows)
	if err != nil {
		return Profile{}, err
	}
	if len(list) == 0 {
		return Profile{}, ErrNotFound
	}
	return list[0], nil
}

func GetSelf(ctx context.Context, q db.Querier, userID int64) (Profile, error) {
	rows, err := q.QueryContext(ctx, selectProfiles+` WHERE p.user_id = ?`, userID)
	if err != nil {
		return Profile{}, fmt.Errorf("get self profile: %w", err)
	}
	list, err := scanProfiles(rows)
	if err != nil {
		return Profile{}, err
	}
	if len(list) == 0 {
		return Profile{}, ErrNotFound
	}
	return list[0], nil
}

// ListVisible returns every profile the subject may read, ordered: self first, then
// profiles the subject manages, then the rest of the family.
func ListVisible(ctx context.Context, q db.Querier, s authz.Subject) ([]Profile, error) {
	rows, err := q.QueryContext(ctx, selectProfiles+`
		WHERE p.user_id = ? OR p.manager_user_id = ? OR (? <> 0 AND u.family_id = ?)
		ORDER BY CASE WHEN p.user_id = ? THEN 0 WHEN p.manager_user_id = ? THEN 1 ELSE 2 END, p.id`,
		s.UserID, s.UserID, s.FamilyID, s.FamilyID, s.UserID, s.UserID)
	if err != nil {
		return nil, fmt.Errorf("list profiles: %w", err)
	}
	return scanProfiles(rows)
}

func scanProfiles(rows *sql.Rows) ([]Profile, error) {
	defer rows.Close()
	var out []Profile
	for rows.Next() {
		var (
			p         Profile
			createdAt int64
		)
		if err := rows.Scan(&p.ID, &p.DisplayName, &p.UserID, &p.ManagerUserID, &p.ManagerName,
			&p.Gender, &p.BirthYear, &p.MedicalHistory, &p.Medications, &p.OwnerFamilyID, &createdAt); err != nil {
			return nil, fmt.Errorf("scan profile: %w", err)
		}
		p.CreatedAt = time.Unix(createdAt, 0)
		out = append(out, p)
	}
	return out, rows.Err()
}

func requireAffected(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func nullInt(v int) any {
	if v == 0 {
		return nil
	}
	return v
}
