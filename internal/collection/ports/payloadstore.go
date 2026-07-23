package ports

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// PayloadSchemeFile is the MVP object-key scheme (ADR-011/019). Promotion to
// S3 reuses the same attribute with an s3:// scheme prefix.
const PayloadSchemeFile = "file://"

// PayloadStore persists raw provider payloads (gzip on a block volume at MVP).
// Keys are scheme-prefixed; the filesystem backend strips the scheme and
// resolves the remainder against its root directory.
type PayloadStore interface {
	// Write gzip-compresses raw and stores it under objectKey.
	Write(ctx context.Context, objectKey string, raw []byte) error
	// Read returns the decompressed payload for objectKey.
	Read(ctx context.Context, objectKey string) ([]byte, error)
	// Quarantine moves a corrupt payload aside (checksum mismatch during
	// replay; FC-14) so it is not served again, returning the new object key.
	// The original stored bytes are preserved for forensic inspection.
	Quarantine(ctx context.Context, objectKey string) (quarantineKey string, err error)
}

// Checksum returns the SHA-256 hex digest of raw. Computed on the raw
// response bytes before parsing (workflow step 7) for lineage integrity.
func Checksum(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// VerifyChecksum reports whether raw matches the recorded SHA-256 hex digest.
func VerifyChecksum(raw []byte, checksum string) bool {
	return Checksum(raw) == checksum
}

// BuildPayloadKey produces the scheme-prefixed object key for a collection's
// raw payload: file://{provider}/{yyyy}/{mm}/{dd}/{collection_id}.json.gz.
func BuildPayloadKey(providerSlug string, at time.Time, collectionID uuid.UUID) string {
	at = at.UTC()
	return fmt.Sprintf("%s%s/%04d/%02d/%02d/%s.json.gz",
		PayloadSchemeFile, providerSlug, at.Year(), int(at.Month()), at.Day(), collectionID)
}
