package record

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrNotFound   = errors.New("record not found")
	ErrValidation = errors.New("validation failed")
)

// ValidationError describes a single field-level validation failure.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationErrors is a collection of ValidationError values that also
// implements the error interface. It is always wrapped with ErrValidation so
// callers can use errors.Is(err, ErrValidation) and then errors.As to extract
// the structured detail.
type ValidationErrors struct {
	Errors []ValidationError
}

func (ve ValidationErrors) Error() string {
	msgs := make([]string, len(ve.Errors))
	for i, e := range ve.Errors {
		msgs[i] = fmt.Sprintf("%s: %s", e.Field, e.Message)
	}
	return strings.Join(msgs, "; ")
}

// Unwrap returns ErrValidation so errors.Is(err, ErrValidation) works when the
// ValidationErrors value itself is the sentinel.
func (ve ValidationErrors) Unwrap() error {
	return ErrValidation
}
