package jwks

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/forecastiq/forecastiq/internal/identity/domain"
)

// fakeClock is a controllable clock for expiry tests.
type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// signRS256 builds a compact RS256 JWT for the given claims and kid.
func signRS256(t *testing.T, priv *rsa.PrivateKey, kid string, claims map[string]any) string {
	t.Helper()
	return signJWT(t, "RS256", kid, claims, func(signingInput string) []byte {
		digest := sha256.Sum256([]byte(signingInput))
		sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, digest[:])
		if err != nil {
			t.Fatalf("sign RS256: %v", err)
		}
		return sig
	})
}

// signES256 builds a compact ES256 JWT (raw r||s signature).
func signES256(t *testing.T, priv *ecdsa.PrivateKey, kid string, claims map[string]any) string {
	t.Helper()
	return signJWT(t, "ES256", kid, claims, func(signingInput string) []byte {
		digest := sha256.Sum256([]byte(signingInput))
		r, s, err := ecdsa.Sign(rand.Reader, priv, digest[:])
		if err != nil {
			t.Fatalf("sign ES256: %v", err)
		}
		sig := make([]byte, 64)
		r.FillBytes(sig[:32])
		s.FillBytes(sig[32:])
		return sig
	})
}

func signJWT(t *testing.T, alg, kid string, claims map[string]any, sign func(string) []byte) string {
	t.Helper()
	header, _ := json.Marshal(map[string]any{"alg": alg, "kid": kid, "typ": "JWT"})
	payload, _ := json.Marshal(claims)
	signingInput := b64(header) + "." + b64(payload)
	return signingInput + "." + b64(sign(signingInput))
}

// rsaJWKS returns a one-key JWKS document for an RSA public key.
func rsaJWKS(kid string, pub *rsa.PublicKey) []byte {
	doc := map[string]any{"keys": []map[string]any{{
		"kty": "RSA", "kid": kid, "alg": "RS256",
		"n": b64(pub.N.Bytes()), "e": b64(big.NewInt(int64(pub.E)).Bytes()),
	}}}
	b, _ := json.Marshal(doc)
	return b
}

func ecJWKS(kid string, pub *ecdsa.PublicKey) []byte {
	x := make([]byte, 32)
	y := make([]byte, 32)
	pub.X.FillBytes(x)
	pub.Y.FillBytes(y)
	doc := map[string]any{"keys": []map[string]any{{
		"kty": "EC", "kid": kid, "crv": "P-256", "x": b64(x), "y": b64(y),
	}}}
	b, _ := json.Marshal(doc)
	return b
}

const (
	testIssuer   = "https://proj.supabase.co/auth/v1"
	testAudience = "authenticated"
)

func baseClaims(exp time.Time) map[string]any {
	return map[string]any{
		"sub": "user-123", "email": "u@example.com",
		"iss": testIssuer, "aud": testAudience, "exp": exp.Unix(),
	}
}

// newTestVerifier serves a mutable JWKS body and returns the verifier + a
// setter to rotate the served keys.
func newTestVerifier(t *testing.T, clk *fakeClock, initial []byte) (*Verifier, func([]byte)) {
	t.Helper()
	var body atomic.Value
	body.Store(initial)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body.Load().([]byte))
	}))
	t.Cleanup(srv.Close)
	v := New(Config{
		JWKSURL: srv.URL, Issuer: testIssuer, Audience: testAudience,
		Clock: clk, MinRefreshInterval: time.Nanosecond,
	})
	return v, func(b []byte) { body.Store(b) }
}

func TestVerify_ValidRS256(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	clk := &fakeClock{now: time.Unix(1_000_000, 0)}
	v, _ := newTestVerifier(t, clk, rsaJWKS("k1", &priv.PublicKey))

	tok := signRS256(t, priv, "k1", baseClaims(clk.now.Add(time.Hour)))
	claims, err := v.Verify(context.Background(), tok)
	if err != nil {
		t.Fatalf("Verify valid: %v", err)
	}
	if claims.Subject != "user-123" || claims.Email != "u@example.com" {
		t.Errorf("claims = %+v", claims)
	}
}

func TestVerify_ValidES256(t *testing.T) {
	priv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	clk := &fakeClock{now: time.Unix(1_000_000, 0)}
	v, _ := newTestVerifier(t, clk, ecJWKS("ec1", &priv.PublicKey))

	tok := signES256(t, priv, "ec1", baseClaims(clk.now.Add(time.Hour)))
	if _, err := v.Verify(context.Background(), tok); err != nil {
		t.Fatalf("Verify ES256 valid: %v", err)
	}
}

func TestVerify_Expired(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	clk := &fakeClock{now: time.Unix(1_000_000, 0)}
	v, _ := newTestVerifier(t, clk, rsaJWKS("k1", &priv.PublicKey))

	tok := signRS256(t, priv, "k1", baseClaims(clk.now.Add(-2*time.Hour)))
	if _, err := v.Verify(context.Background(), tok); !errors.Is(err, domain.ErrTokenExpired) {
		t.Fatalf("expired token err = %v; want ErrTokenExpired", err)
	}
}

func TestVerify_WrongIssuer(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	clk := &fakeClock{now: time.Unix(1_000_000, 0)}
	v, _ := newTestVerifier(t, clk, rsaJWKS("k1", &priv.PublicKey))

	claims := baseClaims(clk.now.Add(time.Hour))
	claims["iss"] = "https://evil.example/auth"
	tok := signRS256(t, priv, "k1", claims)
	if _, err := v.Verify(context.Background(), tok); !errors.Is(err, domain.ErrInvalidToken) {
		t.Fatalf("wrong issuer err = %v; want ErrInvalidToken", err)
	}
}

func TestVerify_WrongAudience(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	clk := &fakeClock{now: time.Unix(1_000_000, 0)}
	v, _ := newTestVerifier(t, clk, rsaJWKS("k1", &priv.PublicKey))

	claims := baseClaims(clk.now.Add(time.Hour))
	claims["aud"] = "someone-else"
	tok := signRS256(t, priv, "k1", claims)
	if _, err := v.Verify(context.Background(), tok); !errors.Is(err, domain.ErrInvalidToken) {
		t.Fatalf("wrong audience err = %v; want ErrInvalidToken", err)
	}
}

func TestVerify_UnknownKID(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	clk := &fakeClock{now: time.Unix(1_000_000, 0)}
	v, _ := newTestVerifier(t, clk, rsaJWKS("k1", &priv.PublicKey))

	tok := signRS256(t, priv, "unknown-kid", baseClaims(clk.now.Add(time.Hour)))
	if _, err := v.Verify(context.Background(), tok); !errors.Is(err, domain.ErrInvalidToken) {
		t.Fatalf("unknown kid err = %v; want ErrInvalidToken", err)
	}
}

func TestVerify_BadSignature(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	clk := &fakeClock{now: time.Unix(1_000_000, 0)}
	// JWKS advertises priv's public key, but the token is signed by `other`.
	v, _ := newTestVerifier(t, clk, rsaJWKS("k1", &priv.PublicKey))

	tok := signRS256(t, other, "k1", baseClaims(clk.now.Add(time.Hour)))
	if _, err := v.Verify(context.Background(), tok); !errors.Is(err, domain.ErrInvalidToken) {
		t.Fatalf("bad signature err = %v; want ErrInvalidToken", err)
	}
}

func TestVerify_Malformed(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	clk := &fakeClock{now: time.Unix(1_000_000, 0)}
	v, _ := newTestVerifier(t, clk, rsaJWKS("k1", &priv.PublicKey))
	for _, bad := range []string{"", "a.b", "not-a-jwt", "a.b.c.d"} {
		if _, err := v.Verify(context.Background(), bad); !errors.Is(err, domain.ErrInvalidToken) {
			t.Errorf("Verify(%q) = %v; want ErrInvalidToken", bad, err)
		}
	}
}

// TestVerify_KeyRotation proves a token signed by a rotated-in key verifies:
// the unknown kid triggers a JWKS refresh that discovers the new key.
func TestVerify_KeyRotation(t *testing.T) {
	old, _ := rsa.GenerateKey(rand.Reader, 2048)
	clk := &fakeClock{now: time.Unix(1_000_000, 0)}
	v, rotate := newTestVerifier(t, clk, rsaJWKS("old", &old.PublicKey))

	// Prime the cache with the old key.
	if _, err := v.Verify(context.Background(), signRS256(t, old, "old", baseClaims(clk.now.Add(time.Hour)))); err != nil {
		t.Fatalf("prime: %v", err)
	}

	// Provider rotates to a new key; a token under the new kid must verify.
	// Advance the clock past the refresh rate-limit window (rotation is a
	// real-time event).
	newKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	rotate(rsaJWKS("new", &newKey.PublicKey))
	clk.now = clk.now.Add(time.Second)
	tok := signRS256(t, newKey, "new", baseClaims(clk.now.Add(time.Hour)))
	if _, err := v.Verify(context.Background(), tok); err != nil {
		t.Fatalf("post-rotation verify: %v", err)
	}
}
