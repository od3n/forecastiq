//go:build release

// Package devauth: release-build stub. In release builds the dev verifier is
// compiled out and any attempt to use it fails closed (ErrInvalidToken), so a
// misconfiguration can never grant unauthenticated access in production.
package devauth

import (
	"context"

	"github.com/forecastiq/forecastiq/internal/identity/domain"
	"github.com/forecastiq/forecastiq/internal/identity/ports"
	"github.com/forecastiq/forecastiq/internal/platform/clock"
)

// Verifier is the disabled release stub.
type Verifier struct{}

// New returns the disabled dev verifier (release builds).
func New(_ clock.Clock) *Verifier { return &Verifier{} }

// Verify always rejects: dev-mode auth is not available in release builds.
func (v *Verifier) Verify(context.Context, string) (*ports.Claims, error) {
	return nil, domain.ErrInvalidToken
}
