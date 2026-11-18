// Package authz is the single place that decides who may read or write a profile's data.
// Handlers must not re-implement these rules.
package authz

// Subject is the logged-in user asking for access.
type Subject struct {
	UserID   int64
	FamilyID int64 // 0 when not in a family
}

// Target describes the profile being accessed.
type Target struct {
	UserID        int64 // account that owns the profile; 0 for a managed profile
	ManagerUserID int64 // account that manages the profile; 0 for a self profile
	OwnerFamilyID int64 // family of the owning or managing account; 0 when none
}

// CanWrite: only the profile's own account or its manager may change its data.
func CanWrite(s Subject, t Target) bool {
	if s.UserID == 0 {
		return false
	}
	return t.UserID == s.UserID || t.ManagerUserID == s.UserID
}

// CanRead: writers, plus anyone in the same family as the owner or manager (read-only).
func CanRead(s Subject, t Target) bool {
	if CanWrite(s, t) {
		return true
	}
	return s.FamilyID != 0 && s.FamilyID == t.OwnerFamilyID
}
