package domain

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Sentinel domain errors. The API layer maps these to RFC 7807 classes.
var (
	// ErrNotFound indicates an unknown resource (404; never distinguish from
	// forbidden-on-private to avoid enumeration).
	ErrNotFound = errors.New("resource not found")
	// ErrInactive indicates a resource exists but is not active.
	ErrInactive = errors.New("resource is not active")
)

// ValidationError carries one or more field-level constraint violations (422).
type ValidationError struct {
	Fields []FieldError
}

// FieldError is a single field constraint violation.
type FieldError struct {
	Field   string
	Message string
}

// Error implements error.
func (e *ValidationError) Error() string {
	parts := make([]string, len(e.Fields))
	for i, f := range e.Fields {
		parts[i] = fmt.Sprintf("%s: %s", f.Field, f.Message)
	}
	return "validation failed: " + strings.Join(parts, "; ")
}

// Add appends a field error.
func (e *ValidationError) Add(field, message string) {
	e.Fields = append(e.Fields, FieldError{Field: field, Message: message})
}

// HasErrors reports whether any field errors were recorded.
func (e *ValidationError) HasErrors() bool { return len(e.Fields) > 0 }

// ErrorOrNil returns the ValidationError as an error, or nil when empty.
func (e *ValidationError) ErrorOrNil() error {
	if e.HasErrors() {
		return e
	}
	return nil
}

// DuplicateLocationError signals a BR-LOC-01 near-duplicate (409 duplicate).
// It carries the existing location reference and the angular distance so the
// client can present the resolution path.
type DuplicateLocationError struct {
	ExistingID      uuid.UUID
	ExistingName    string
	DistanceDegrees float64
}

// Error implements error.
func (e *DuplicateLocationError) Error() string {
	return fmt.Sprintf("location %.6f° from existing location %q (id=%s); near-duplicate (BR-LOC-01)",
		e.DistanceDegrees, e.ExistingName, e.ExistingID)
}
