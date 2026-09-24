package httpapi

import "errors"

// Sentinel bind/query errors (safe to expose as detail).
var (
	ErrInvalidJSON  = errors.New("request body is not valid JSON")
	ErrInvalidQuery = errors.New("query parameters are invalid")
	ErrUnknownField = errors.New("request contains unknown fields")
	ErrBodyTooLarge = errors.New("request body is too large")
	ErrEmptyBody    = errors.New("request body is required")
	ErrValidation   = errors.New("validation failed")
)

// ValidationError wraps field-level validation failures.
type ValidationError struct {
	Fields []FieldError
}

func (e *ValidationError) Error() string { return ErrValidation.Error() }
func (e *ValidationError) Unwrap() error { return ErrValidation }
