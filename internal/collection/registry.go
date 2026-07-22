package collection

import (
	"fmt"
	"sort"

	"github.com/forecastiq/forecastiq/internal/collection/ports"
)

// ProviderDescriptor is the operational view of a registered adapter: identity,
// recorded versions, and declared capabilities. Emitted for startup inspection
// and available to future admin introspection (no HTTP route in WP-05).
type ProviderDescriptor struct {
	Slug           string
	SchemaVersion  string
	AdapterVersion string
	Capabilities   ports.Capabilities
}

// Registry is the provider-agnostic adapter registry used by the composition
// root. It validates identity/versions and rejects duplicate slugs so a
// misconfigured wiring fails fast at startup rather than at first collection.
type Registry struct {
	byslug map[string]ports.ForecastProviderAdapter
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{byslug: make(map[string]ports.ForecastProviderAdapter)}
}

// Register validates and adds an adapter. It errors on an empty slug/version or
// a duplicate slug.
func (r *Registry) Register(a ports.ForecastProviderAdapter) error {
	if a == nil {
		return fmt.Errorf("provider registry: nil adapter")
	}
	slug := a.Slug()
	switch {
	case slug == "":
		return fmt.Errorf("provider registry: adapter has empty slug")
	case a.SchemaVersion() == "":
		return fmt.Errorf("provider registry: adapter %q has empty schema version", slug)
	case a.AdapterVersion() == "":
		return fmt.Errorf("provider registry: adapter %q has empty adapter version", slug)
	}
	if _, dup := r.byslug[slug]; dup {
		return fmt.Errorf("provider registry: adapter %q already registered", slug)
	}
	if a.Capabilities().SupportsReplay {
		if _, ok := a.(ports.ReplayDecoder); !ok {
			return fmt.Errorf("provider registry: adapter %q declares replay support but does not implement ReplayDecoder", slug)
		}
	}
	r.byslug[slug] = a
	return nil
}

// Get returns the adapter for slug.
func (r *Registry) Get(slug string) (ports.ForecastProviderAdapter, bool) {
	a, ok := r.byslug[slug]
	return a, ok
}

// Adapters returns a copy of the slug→adapter map for wiring into the collector.
func (r *Registry) Adapters() map[string]ports.ForecastProviderAdapter {
	out := make(map[string]ports.ForecastProviderAdapter, len(r.byslug))
	for k, v := range r.byslug {
		out[k] = v
	}
	return out
}

// Len returns the number of registered adapters.
func (r *Registry) Len() int { return len(r.byslug) }

// Descriptors returns the registered adapters as descriptors, sorted by slug.
func (r *Registry) Descriptors() []ProviderDescriptor {
	out := make([]ProviderDescriptor, 0, len(r.byslug))
	for _, a := range r.byslug {
		out = append(out, ProviderDescriptor{
			Slug:           a.Slug(),
			SchemaVersion:  a.SchemaVersion(),
			AdapterVersion: a.AdapterVersion(),
			Capabilities:   a.Capabilities(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out
}
