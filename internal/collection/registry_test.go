package collection_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/forecastiq/forecastiq/internal/collection"
	"github.com/forecastiq/forecastiq/internal/collection/ports"
)

// stubAdapter is a minimal adapter for registry tests. When replay is true it
// also satisfies ports.ReplayDecoder.
type stubAdapter struct {
	slug    string
	schema  string
	version string
	caps    ports.Capabilities
}

func (s *stubAdapter) Slug() string                     { return s.slug }
func (s *stubAdapter) SchemaVersion() string            { return s.schema }
func (s *stubAdapter) AdapterVersion() string           { return s.version }
func (s *stubAdapter) Capabilities() ports.Capabilities { return s.caps }
func (s *stubAdapter) FetchForecast(context.Context, ports.ForecastRequest) (*ports.ForecastResult, error) {
	return &ports.ForecastResult{Outcome: ports.OutcomeSuccess}, nil
}

// replayStub embeds stubAdapter and implements ReplayDecoder.
type replayStub struct{ stubAdapter }

func (r *replayStub) DecodeStored(context.Context, ports.ForecastRequest, []byte) (*ports.ForecastResult, error) {
	return &ports.ForecastResult{Outcome: ports.OutcomeSuccess}, nil
}

func valid(slug string) *stubAdapter {
	return &stubAdapter{slug: slug, schema: slug + "-v1", version: "1.0.0"}
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := collection.NewRegistry()
	require.NoError(t, r.Register(valid("open-meteo")))
	assert.Equal(t, 1, r.Len())

	a, ok := r.Get("open-meteo")
	require.True(t, ok)
	assert.Equal(t, "open-meteo", a.Slug())

	_, ok = r.Get("missing")
	assert.False(t, ok)
}

func TestRegistry_RejectsDuplicateSlug(t *testing.T) {
	r := collection.NewRegistry()
	require.NoError(t, r.Register(valid("open-meteo")))
	err := r.Register(valid("open-meteo"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestRegistry_RejectsEmptyIdentity(t *testing.T) {
	r := collection.NewRegistry()
	assert.Error(t, r.Register(&stubAdapter{slug: "", schema: "x", version: "1"}))
	assert.Error(t, r.Register(&stubAdapter{slug: "p", schema: "", version: "1"}))
	assert.Error(t, r.Register(&stubAdapter{slug: "p", schema: "x", version: ""}))
	assert.Error(t, r.Register(nil))
}

func TestRegistry_ReplayCapabilityMustBeImplemented(t *testing.T) {
	r := collection.NewRegistry()
	// Declares replay support but does NOT implement ReplayDecoder → rejected.
	bad := valid("declares-replay")
	bad.caps = ports.Capabilities{SupportsReplay: true}
	err := r.Register(bad)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ReplayDecoder")

	// Declares replay AND implements it → accepted.
	good := &replayStub{stubAdapter{slug: "good", schema: "good-v1", version: "1.0.0",
		caps: ports.Capabilities{SupportsReplay: true}}}
	require.NoError(t, r.Register(good))
}

func TestRegistry_DescriptorsSorted(t *testing.T) {
	r := collection.NewRegistry()
	require.NoError(t, r.Register(&stubAdapter{slug: "zeta", schema: "z-v1", version: "1.0.0",
		caps: ports.Capabilities{MaxForecastHorizon: 24 * time.Hour}}))
	require.NoError(t, r.Register(valid("alpha")))

	ds := r.Descriptors()
	require.Len(t, ds, 2)
	assert.Equal(t, "alpha", ds[0].Slug)
	assert.Equal(t, "zeta", ds[1].Slug)
	assert.Equal(t, 24*time.Hour, ds[1].Capabilities.MaxForecastHorizon)
}

func TestRegistry_AdaptersReturnsCopy(t *testing.T) {
	r := collection.NewRegistry()
	require.NoError(t, r.Register(valid("open-meteo")))
	m := r.Adapters()
	delete(m, "open-meteo") // mutating the copy must not affect the registry
	_, ok := r.Get("open-meteo")
	assert.True(t, ok)
}
