package handlers

import (
	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/api/respond"
	"github.com/forecastiq/forecastiq/internal/catalog"
)

// Warning codes (closed enum; conventions §3 / caching §4.2).
const warnProviderUnavailable = "provider_unavailable"

// absentProviderWarnings builds the partial-result warnings for active
// providers that are missing from a derived payload (caching doc §4.1: a
// collection gap for the requested view ⇒ provider_unavailable; affected
// providers are omitted from the data arrays and surfaced here instead).
//
// It deliberately returns no warnings when nothing is servable (present is
// empty): an all-providers-absent response is NOT a partial result (§4.2 rule
// 6) — the freshness=unavailable block communicates that case instead.
func absentProviderWarnings(active []*catalog.Provider, present map[uuid.UUID]bool) []respond.Warning {
	if len(present) == 0 {
		return nil
	}
	var warnings []respond.Warning
	for _, p := range active {
		if p.Status != catalog.StatusActive || present[p.ID] {
			continue
		}
		warnings = append(warnings, respond.Warning{
			ProviderID: p.ID.String(),
			Code:       warnProviderUnavailable,
			Message:    "No published data for this provider in the requested view.",
		})
	}
	return warnings
}
