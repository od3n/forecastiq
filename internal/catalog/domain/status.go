// Package domain holds the catalog module's pure domain model: entities,
// value objects, invariants, and domain errors. It imports only the standard
// library and the UUID kernel (ADR-022) — no infrastructure (binding rule:
// domain has zero infrastructure imports).
package domain

// Status is the lifecycle state of mutable catalog entities.
type Status string

const (
	StatusActive   Status = "active"
	StatusDisabled Status = "disabled"
	StatusArchived Status = "archived"
)

// Valid reports whether the status is a known value.
func (s Status) Valid() bool {
	switch s {
	case StatusActive, StatusDisabled, StatusArchived:
		return true
	}
	return false
}

// Active reports whether the entity participates in operations.
func (s Status) Active() bool { return s == StatusActive }
