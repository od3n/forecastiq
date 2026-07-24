package handlers

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/forecastiq/forecastiq/internal/catalog"
	catalogdomain "github.com/forecastiq/forecastiq/internal/catalog/domain"
)

func activeProvider(name string, status catalog.Status) *catalog.Provider {
	return &catalogdomain.Provider{ID: uuid.New(), Name: name, Slug: name, Status: status}
}

func TestAbsentProviderWarnings(t *testing.T) {
	om := activeProvider("open-meteo", catalog.StatusActive)
	ow := activeProvider("openweather", catalog.StatusActive)
	disabled := activeProvider("legacy", catalog.StatusDisabled)
	active := []*catalog.Provider{om, ow, disabled}

	// OM present, OW absent (active) → single provider_unavailable warning;
	// the disabled provider is never warned about.
	w := absentProviderWarnings(active, map[uuid.UUID]bool{om.ID: true})
	require.Len(t, w, 1)
	assert.Equal(t, ow.ID.String(), w[0].ProviderID)
	assert.Equal(t, warnProviderUnavailable, w[0].Code)
}

func TestAbsentProviderWarnings_AllPresent(t *testing.T) {
	om := activeProvider("open-meteo", catalog.StatusActive)
	w := absentProviderWarnings([]*catalog.Provider{om}, map[uuid.UUID]bool{om.ID: true})
	assert.Empty(t, w)
}

func TestAbsentProviderWarnings_NothingServableIsNotPartial(t *testing.T) {
	om := activeProvider("open-meteo", catalog.StatusActive)
	// Empty present set → all-absent is NOT a partial result (§4.2 rule 6).
	w := absentProviderWarnings([]*catalog.Provider{om}, map[uuid.UUID]bool{})
	assert.Empty(t, w)
}
