package payloadstore_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/forecastiq/forecastiq/adapters/payloadstore"
	"github.com/forecastiq/forecastiq/internal/collection/ports"
)

func TestFilesystemStore_WriteReadRoundtrip(t *testing.T) {
	store, err := payloadstore.NewFilesystemStore(t.TempDir())
	require.NoError(t, err)

	key := ports.BuildPayloadKey("open-meteo", time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC), uuid.New())
	payload := []byte(`{"hourly":{"time":["2026-07-22T11:00"]}}`)

	require.NoError(t, store.Write(context.Background(), key, payload))
	got, err := store.Read(context.Background(), key)
	require.NoError(t, err)
	assert.Equal(t, payload, got)
}

func TestFilesystemStore_ChecksumVerifies(t *testing.T) {
	payload := []byte(`{"a":1}`)
	sum := ports.Checksum(payload)
	assert.True(t, ports.VerifyChecksum(payload, sum))
	assert.False(t, ports.VerifyChecksum([]byte(`{"a":2}`), sum))
}

func TestFilesystemStore_ReadMissing(t *testing.T) {
	store, err := payloadstore.NewFilesystemStore(t.TempDir())
	require.NoError(t, err)
	_, err = store.Read(context.Background(), "file://open-meteo/2026/07/22/does-not-exist.json.gz")
	assert.Error(t, err)
}

func TestFilesystemStore_RejectsTraversal(t *testing.T) {
	store, err := payloadstore.NewFilesystemStore(t.TempDir())
	require.NoError(t, err)
	err = store.Write(context.Background(), "file://../../etc/passwd", []byte("x"))
	assert.Error(t, err)
}

func TestFilesystemStore_Writable(t *testing.T) {
	store, err := payloadstore.NewFilesystemStore(t.TempDir())
	require.NoError(t, err)
	assert.NoError(t, store.Writable())
}
