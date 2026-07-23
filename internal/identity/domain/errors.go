package domain

import "errors"

// Sentinel domain errors, mapped to RFC 7807 classes by the API layer
// (handlers land in WP-15/WP-19; the mappings live with them).
var (
	// ErrUserNotFound is returned when no user matches the lookup.
	ErrUserNotFound = errors.New("user not found")
	// ErrUserDisabled is returned when a disabled/archived user attempts to
	// authenticate (role/status come from the database, not the token).
	ErrUserDisabled = errors.New("user account is not active")
	// ErrKeyNotFound is returned when no API key matches (also used to avoid
	// leaking existence to a non-owner).
	ErrKeyNotFound = errors.New("api key not found")
	// ErrKeyInactive is returned when a presented key is revoked or expired.
	ErrKeyInactive = errors.New("api key is revoked or expired")
	// ErrInvalidToken is returned when a bearer token fails verification
	// (bad signature, unknown kid, wrong issuer/audience, malformed).
	ErrInvalidToken = errors.New("invalid authentication token")
	// ErrTokenExpired is returned when a token is well-formed but expired.
	ErrTokenExpired = errors.New("authentication token expired")
	// ErrInvalidCredential is returned when a presented API key secret does
	// not match the stored hash.
	ErrInvalidCredential = errors.New("invalid credential")
	// ErrDuplicateSubject is returned by the repository when an insert races
	// with a concurrent provisioning of the same auth subject (unique
	// violation); the service recovers by re-reading the existing user.
	ErrDuplicateSubject = errors.New("user already provisioned for auth subject")
)
