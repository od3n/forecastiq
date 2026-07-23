package identity

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// API-key format: "<prefix>.<secret>" where prefix = "fiq_" + 8 random hex
// chars (the non-secret lookup handle) and secret is 32 random bytes,
// base64url-encoded. Only the argon2id hash of the full presented string is
// stored (never the plaintext).
const (
	keyPrefixLabel = "fiq_"
	keyPrefixBytes = 4  // → 8 hex chars
	keySecretBytes = 32 // 256-bit secret
)

// argon2id parameters (OWASP-aligned baseline). Encoded into the PHC hash so a
// future parameter bump verifies old hashes and re-hashes on next use.
const (
	argonMemoryKiB = 64 * 1024 // 64 MiB
	argonTime      = 1
	argonThreads   = 4
	argonKeyLen    = 32
	argonSaltLen   = 16
)

// generateKey returns a fresh (prefix, plaintext) pair. plaintext is
// "<prefix>.<secret>" and is shown to the user exactly once.
func generateKey() (prefix, plaintext string, err error) {
	pb := make([]byte, keyPrefixBytes)
	if _, err = rand.Read(pb); err != nil {
		return "", "", fmt.Errorf("generate key prefix: %w", err)
	}
	sb := make([]byte, keySecretBytes)
	if _, err = rand.Read(sb); err != nil {
		return "", "", fmt.Errorf("generate key secret: %w", err)
	}
	prefix = keyPrefixLabel + hex.EncodeToString(pb)
	secret := base64.RawURLEncoding.EncodeToString(sb)
	return prefix, prefix + "." + secret, nil
}

// splitKey extracts the lookup prefix from a presented plaintext key.
func splitKey(plaintext string) (prefix string, ok bool) {
	i := strings.IndexByte(plaintext, '.')
	if i <= 0 || !strings.HasPrefix(plaintext, keyPrefixLabel) {
		return "", false
	}
	return plaintext[:i], true
}

// hashKey returns a PHC-encoded argon2id hash of the full plaintext key.
func hashKey(plaintext string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	sum := argon2.IDKey([]byte(plaintext), salt, argonTime, argonMemoryKiB, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemoryKiB, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(sum)), nil
}

// verifyKey reports whether plaintext matches the PHC-encoded argon2id hash,
// in constant time. A malformed hash yields (false, error).
func verifyKey(plaintext, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errors.New("unsupported key hash format")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, errors.New("unsupported argon2 version")
	}
	var mem uint32
	var t, p uint32
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &mem, &t, &p); err != nil {
		return false, errors.New("malformed argon2 parameters")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("decode salt: %w", err)
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("decode hash: %w", err)
	}
	got := argon2.IDKey([]byte(plaintext), salt, t, mem, uint8(p), uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
