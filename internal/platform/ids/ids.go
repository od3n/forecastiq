// Package ids provides identifier generation per ADR-022 (UUIDv7 everywhere).
// UUIDv7 is time-ordered, giving bigint-like B-tree behaviour on the hot
// insert path while preserving global uniqueness and allowing client-side
// (pre-tx) generation — required for idempotent batch snapshot writes.
package ids

import "github.com/google/uuid"

// New returns a fresh UUIDv7. It panics only on entropy exhaustion, which
// uuid.NewV7 documents as effectively impossible on supported platforms.
func New() uuid.UUID {
	return uuid.Must(uuid.NewV7())
}

// Parse validates and parses a canonical hyphenated UUID string.
func Parse(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}
