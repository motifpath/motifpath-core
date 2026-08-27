package domain

import (
	"errors"
	"strings"
)

var (
	// ErrNotFound is returned when a requested resource does not exist.
	ErrNotFound = errors.New("not found")

	// ErrForbidden is returned when the authenticated caller's role does not
	// permit the requested operation.
	ErrForbidden = errors.New("forbidden")

	// ErrAlreadyExists is returned when a resource that must be unique
	// (e.g. a user record for a Clerk identity) already exists.
	ErrAlreadyExists = errors.New("already exists")

	// ErrValidation is the sentinel every *ValidationError wraps. Callers
	// that only need to know "was this a validation failure" can test with
	// errors.Is(err, domain.ErrValidation); callers that need the
	// field-level detail for the HTTP ValidationError response body use
	// errors.As to recover the *ValidationError itself.
	ErrValidation = errors.New("validation failed")
)

// FieldError names one request field that failed validation and why.
type FieldError struct {
	Field  string
	Reason string
}

// ValidationError carries one or more FieldErrors, matching the shape of
// the OpenAPI ValidationError response (message + one entry per failing
// field).
type ValidationError struct {
	Fields []FieldError
}

// NewValidationError constructs a ValidationError with a single field
// failure — the common case for domain constructors that stop at the first
// invariant violation.
func NewValidationError(field, reason string) *ValidationError {
	return &ValidationError{Fields: []FieldError{{Field: field, Reason: reason}}}
}

func (e *ValidationError) Error() string {
	parts := make([]string, len(e.Fields))
	for i, f := range e.Fields {
		parts[i] = f.Field + ": " + f.Reason
	}
	return "validation failed: " + strings.Join(parts, "; ")
}

func (e *ValidationError) Unwrap() error {
	return ErrValidation
}
