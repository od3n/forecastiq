//go:build !release

// Package devauth provides a LOCAL-ONLY TokenVerifier so developers can
// exercise authenticated flows without a Supabase project (ADR-008 dev-mode
// auth). It performs NO cryptographic verification — it trusts the presented
// token as the caller's identity — and is therefore excluded from release
// builds by the `release` build tag (see devauth_release.go). The composition
// root only selects it in non-production environments.
package devauth

import (
	"context"
	"strings"
	"time"

	"github.com/forecastiq/forecastiq/internal/identity/domain"
	"github.com/forecastiq/forecastiq/internal/identity/ports"
	"github.com/forecastiq/forecastiq/internal/platform/clock"
)

// Verifier is the development token verifier. The token format is
// "<subject>" or "<subject>:<email>"; the subject maps to users.auth_subject
// so provisioning and lookups behave exactly as with a real JWT.
type Verifier struct {
	clock clock.Clock
	ttl   time.Duration
}

// New builds a dev Verifier.
func New(clk clock.Clock) *Verifier {
	if clk == nil {
		clk = clock.Real{}
	}
	return &Verifier{clock: clk, ttl: time.Hour}
}

// Verify implements ports.TokenVerifier for local development.
func (v *Verifier) Verify(_ context.Context, raw string) (*ports.Claims, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, domain.ErrInvalidToken
	}
	subject, email, _ := strings.Cut(raw, ":")
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return nil, domain.ErrInvalidToken
	}
	if email = strings.TrimSpace(email); email == "" {
		email = subject + "@dev.local"
	}
	return &ports.Claims{Subject: "dev|" + subject, Email: email, ExpiresAt: v.clock.Now().Add(v.ttl)}, nil
}
