// Package validate carries field-level input errors from domain code to the HTTP layer.
package validate

import "fmt"

// Error names the offending field and a machine-readable reason; the frontend maps both
// to user-facing text.
type Error struct {
	Field  string
	Reason string
}

func (e *Error) Error() string {
	return fmt.Sprintf("invalid %s: %s", e.Field, e.Reason)
}

func Field(field, reason string) *Error {
	return &Error{Field: field, Reason: reason}
}
