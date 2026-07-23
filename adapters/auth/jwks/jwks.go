// Package jwks implements the identity module's TokenVerifier port against a
// JWKS endpoint (Supabase Auth; ADR-008). It verifies RS256 and ES256 compact
// JWTs using only the standard library crypto primitives — no third-party JWT
// dependency — and caches the signing keys, refreshing (rotation-tolerant) when
// an unknown key id is presented. It never surfaces cryptographic internals:
// callers receive domain.ErrInvalidToken or domain.ErrTokenExpired.
package jwks

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/forecastiq/forecastiq/internal/identity/domain"
	"github.com/forecastiq/forecastiq/internal/identity/ports"
	"github.com/forecastiq/forecastiq/internal/platform/clock"
)

// Config configures a Verifier.
type Config struct {
	JWKSURL            string
	Issuer             string        // expected `iss` (empty disables the check)
	Audience           string        // expected `aud` (empty disables the check)
	HTTPClient         *http.Client  // defaults to a 10s-timeout client
	Clock              clock.Clock   // defaults to the real clock
	MinRefreshInterval time.Duration // rate-limits refetch on unknown kid (default 1m)
	Leeway             time.Duration // clock-skew allowance for exp (default 60s)
}

// Verifier verifies bearer JWTs against a cached JWKS.
type Verifier struct {
	url        string
	issuer     string
	audience   string
	httpClient *http.Client
	clock      clock.Clock
	minRefresh time.Duration
	leeway     time.Duration

	mu          sync.RWMutex
	keys        map[string]crypto.PublicKey
	lastRefresh time.Time
}

// New builds a Verifier.
func New(cfg Config) *Verifier {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}
	if cfg.Clock == nil {
		cfg.Clock = clock.Real{}
	}
	if cfg.MinRefreshInterval <= 0 {
		cfg.MinRefreshInterval = time.Minute
	}
	if cfg.Leeway <= 0 {
		cfg.Leeway = 60 * time.Second
	}
	return &Verifier{
		url: cfg.JWKSURL, issuer: cfg.Issuer, audience: cfg.Audience,
		httpClient: cfg.HTTPClient, clock: cfg.Clock,
		minRefresh: cfg.MinRefreshInterval, leeway: cfg.Leeway,
		keys: map[string]crypto.PublicKey{},
	}
}

// jwtHeader is the decoded JOSE header.
type jwtHeader struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
}

// jwtClaims is the subset of registered/Supabase claims we consume. Audience
// tolerates both a string and an array (RFC 7519).
type jwtClaims struct {
	Subject  string   `json:"sub"`
	Email    string   `json:"email"`
	Issuer   string   `json:"iss"`
	Expiry   int64    `json:"exp"`
	Audience audience `json:"aud"`
}

// Verify implements ports.TokenVerifier.
func (v *Verifier) Verify(ctx context.Context, raw string) (*ports.Claims, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return nil, domain.ErrInvalidToken
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, domain.ErrInvalidToken
	}
	var header jwtHeader
	if json.Unmarshal(headerBytes, &header) != nil {
		return nil, domain.ErrInvalidToken
	}
	if header.Alg != "RS256" && header.Alg != "ES256" {
		return nil, domain.ErrInvalidToken
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, domain.ErrInvalidToken
	}

	key, err := v.keyForKID(ctx, header.Kid)
	if err != nil {
		return nil, domain.ErrInvalidToken
	}
	signingInput := parts[0] + "." + parts[1]
	if !verifySignature(header.Alg, key, signingInput, sig) {
		return nil, domain.ErrInvalidToken
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, domain.ErrInvalidToken
	}
	var claims jwtClaims
	if json.Unmarshal(payloadBytes, &claims) != nil {
		return nil, domain.ErrInvalidToken
	}
	if claims.Subject == "" {
		return nil, domain.ErrInvalidToken
	}
	if v.issuer != "" && claims.Issuer != v.issuer {
		return nil, domain.ErrInvalidToken
	}
	if v.audience != "" && !claims.Audience.contains(v.audience) {
		return nil, domain.ErrInvalidToken
	}
	if claims.Expiry == 0 {
		return nil, domain.ErrInvalidToken
	}
	exp := time.Unix(claims.Expiry, 0)
	if v.clock.Now().After(exp.Add(v.leeway)) {
		return nil, domain.ErrTokenExpired
	}
	return &ports.Claims{Subject: claims.Subject, Email: claims.Email, ExpiresAt: exp}, nil
}

// keyForKID returns the public key for kid, refreshing the JWKS once (rate
// limited) when the kid is unknown (rotation tolerance).
func (v *Verifier) keyForKID(ctx context.Context, kid string) (crypto.PublicKey, error) {
	v.mu.RLock()
	key, ok := v.keys[kid]
	v.mu.RUnlock()
	if ok {
		return key, nil
	}
	if err := v.refresh(ctx); err != nil {
		return nil, err
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	if key, ok := v.keys[kid]; ok {
		return key, nil
	}
	return nil, fmt.Errorf("unknown key id %q", kid)
}

// refresh refetches the JWKS, rate-limited by MinRefreshInterval.
func (v *Verifier) refresh(ctx context.Context) error {
	v.mu.Lock()
	if !v.lastRefresh.IsZero() && v.clock.Now().Sub(v.lastRefresh) < v.minRefresh {
		v.mu.Unlock()
		return nil
	}
	v.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.url, nil)
	if err != nil {
		return err
	}
	resp, err := v.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jwks endpoint returned %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	keys, err := parseJWKS(body)
	if err != nil {
		return err
	}
	v.mu.Lock()
	v.keys = keys
	v.lastRefresh = v.clock.Now()
	v.mu.Unlock()
	return nil
}

// verifySignature checks the JWS signature for the supported algorithms.
func verifySignature(alg string, key crypto.PublicKey, signingInput string, sig []byte) bool {
	digest := sha256.Sum256([]byte(signingInput))
	switch alg {
	case "RS256":
		pub, ok := key.(*rsa.PublicKey)
		if !ok {
			return false
		}
		return rsa.VerifyPKCS1v15(pub, crypto.SHA256, digest[:], sig) == nil
	case "ES256":
		pub, ok := key.(*ecdsa.PublicKey)
		if !ok || len(sig) != 64 {
			return false
		}
		r := new(big.Int).SetBytes(sig[:32])
		s := new(big.Int).SetBytes(sig[32:])
		return ecdsa.Verify(pub, digest[:], r, s)
	default:
		return false
	}
}

// ── JWKS parsing ──────────────────────────────────────────────────────

type jwksDoc struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

func parseJWKS(body []byte) (map[string]crypto.PublicKey, error) {
	var doc jwksDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("parse jwks: %w", err)
	}
	out := make(map[string]crypto.PublicKey, len(doc.Keys))
	for _, k := range doc.Keys {
		pub, err := k.publicKey()
		if err != nil || k.Kid == "" {
			continue // skip unusable keys rather than fail the whole set
		}
		out[k.Kid] = pub
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("jwks contained no usable keys")
	}
	return out, nil
}

func (k jwk) publicKey() (crypto.PublicKey, error) {
	switch k.Kty {
	case "RSA":
		n, err := base64.RawURLEncoding.DecodeString(k.N)
		if err != nil {
			return nil, err
		}
		e, err := base64.RawURLEncoding.DecodeString(k.E)
		if err != nil {
			return nil, err
		}
		return &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}, nil
	case "EC":
		if k.Crv != "P-256" {
			return nil, fmt.Errorf("unsupported curve %q", k.Crv)
		}
		x, err := base64.RawURLEncoding.DecodeString(k.X)
		if err != nil {
			return nil, err
		}
		y, err := base64.RawURLEncoding.DecodeString(k.Y)
		if err != nil {
			return nil, err
		}
		return &ecdsa.PublicKey{Curve: elliptic.P256(), X: new(big.Int).SetBytes(x), Y: new(big.Int).SetBytes(y)}, nil
	default:
		return nil, fmt.Errorf("unsupported key type %q", k.Kty)
	}
}

// audience decodes a JWT `aud` claim that may be a string or an array.
type audience []string

func (a *audience) UnmarshalJSON(b []byte) error {
	var single string
	if err := json.Unmarshal(b, &single); err == nil {
		*a = []string{single}
		return nil
	}
	var many []string
	if err := json.Unmarshal(b, &many); err != nil {
		return err
	}
	*a = many
	return nil
}

func (a audience) contains(want string) bool {
	for _, v := range a {
		if v == want {
			return true
		}
	}
	return false
}
