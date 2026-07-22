// Package payloadstore implements the raw-payload store port (ADR-011/019):
// gzip-compressed provider responses on a block volume, addressed by
// scheme-prefixed keys (file://… at MVP; s3://… post-promotion). Payloads are
// debugging/lineage aids — never served over HTTP, never logged.
package payloadstore

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/forecastiq/forecastiq/internal/collection/ports"
)

// FilesystemStore stores payloads under a root directory. It implements
// ports.PayloadStore for the file:// scheme.
type FilesystemStore struct {
	root string
}

// NewFilesystemStore returns a store rooted at dir (created on first write).
func NewFilesystemStore(dir string) (*FilesystemStore, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve payload dir: %w", err)
	}
	return &FilesystemStore{root: abs}, nil
}

// Write gzip-compresses raw and stores it under the scheme-prefixed objectKey.
func (s *FilesystemStore) Write(_ context.Context, objectKey string, raw []byte) error {
	path, err := s.resolve(objectKey)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("create payload dir: %w", err)
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(raw); err != nil {
		return fmt.Errorf("gzip payload: %w", err)
	}
	if err := gz.Close(); err != nil {
		return fmt.Errorf("close gzip: %w", err)
	}
	// Write atomically (temp + rename) so readers never see a partial file.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o640); err != nil {
		return fmt.Errorf("write payload: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("finalize payload: %w", err)
	}
	return nil
}

// Read returns the decompressed payload for objectKey.
func (s *FilesystemStore) Read(_ context.Context, objectKey string) ([]byte, error) {
	path, err := s.resolve(objectKey)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", os.ErrNotExist, objectKey)
		}
		return nil, fmt.Errorf("open payload: %w", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("payload corrupt (gzip): %w", err)
	}
	defer gz.Close()
	return io.ReadAll(gz)
}

// Writable reports whether the store root is writable (readiness probe).
func (s *FilesystemStore) Writable() error {
	if err := os.MkdirAll(s.root, 0o750); err != nil {
		return fmt.Errorf("payload volume not writable: %w", err)
	}
	probe := filepath.Join(s.root, ".write-probe")
	if err := os.WriteFile(probe, []byte("ok"), 0o640); err != nil {
		return fmt.Errorf("payload volume not writable: %w", err)
	}
	_ = os.Remove(probe)
	return nil
}

// resolve maps a scheme-prefixed key to an absolute path under the root,
// rejecting any traversal outside the root.
func (s *FilesystemStore) resolve(objectKey string) (string, error) {
	rel := strings.TrimPrefix(objectKey, ports.PayloadSchemeFile)
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" || strings.Contains(rel, "..") {
		return "", fmt.Errorf("invalid payload key: %q", objectKey)
	}
	path := filepath.Join(s.root, filepath.FromSlash(rel))
	if !strings.HasPrefix(path, s.root+string(os.PathSeparator)) {
		return "", fmt.Errorf("payload key escapes store root: %q", objectKey)
	}
	return path, nil
}
