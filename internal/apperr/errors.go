// Package apperr defines sentinel errors shared across the service and
// handler layers, so handlers can map errors to HTTP status codes with
// errors.Is / errors.As instead of string matching.
package apperr

import "errors"

var (
	ErrNotFound           = errors.New("resource not found")
	ErrDuplicateEmail     = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrValidation         = errors.New("validation failed")
	ErrForbidden          = errors.New("forbidden")
)

// ValidationError carries a field-specific message while still satisfying
// errors.Is(err, ErrValidation) via Unwrap.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return e.Message }
func (e *ValidationError) Unwrap() error  { return ErrValidation }

// Validation is a convenience constructor for a ValidationError.
func Validation(msg string) error {
	return &ValidationError{Message: msg}
}
