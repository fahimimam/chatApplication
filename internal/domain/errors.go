// Package domain contains the core business types for the chat application.
//
// The domain layer is intentionally I/O free. It defines entities, value
// objects, invariants, and the sentinel errors that upper layers translate into
// transport-specific responses (HTTP status codes, TUI strings, etc.).
package domain

import "errors"

// Domain errors. Callers compare with errors.Is.
var (
	ErrNotFound          = errors.New("not found")
	ErrAlreadyExists     = errors.New("already exists")
	ErrInvalidCredential = errors.New("invalid credential")
	ErrForbidden         = errors.New("forbidden")
	ErrValidation        = errors.New("validation failed")
	ErrConflict          = errors.New("conflict")
)

// ValidationError carries a per-field reason for callers that want to render
// structured feedback (HTTP JSON error bodies, TUI field hints).
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field == "" {
		return e.Message
	}
	return e.Field + ": " + e.Message
}

// Is allows errors.Is(err, ErrValidation) to match.
func (e *ValidationError) Is(target error) bool {
	return target == ErrValidation
}
