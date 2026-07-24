package respond

import "time"

// Freshness state values (conventions §2; four states only).
const (
	FreshnessFresh       = "fresh"
	FreshnessDelayed     = "delayed"
	FreshnessStale       = "stale"
	FreshnessUnavailable = "unavailable"
)

// ComputeFreshness derives the server-side freshness block from the age of the
// most recent underlying datum (BR-FRESH-02: always server-computed, never
// derived by the client from last_updated alone).
//
//   - lastUpdated nil            → unavailable (optionally with a reason)
//   - age ≤ freshMax             → fresh
//   - freshMax < age ≤ staleMax  → delayed
//   - age > staleMax             → stale
//
// threshold_seconds echoes the fresh→delayed boundary (freshMax) so the UI can
// render honest tooltips (conventions §2).
func ComputeFreshness(lastUpdated *time.Time, now time.Time, freshMax, staleMax time.Duration, unavailableReason string) *Freshness {
	if lastUpdated == nil {
		f := &Freshness{State: FreshnessUnavailable}
		if unavailableReason != "" {
			f.Reason = unavailableReason
		}
		return f
	}
	age := now.Sub(*lastUpdated)
	if age < 0 {
		age = 0
	}
	state := FreshnessStale
	switch {
	case age <= freshMax:
		state = FreshnessFresh
	case age <= staleMax:
		state = FreshnessDelayed
	}
	lu := lastUpdated.UTC()
	return &Freshness{
		State:            state,
		LastUpdated:      &lu,
		AgeSeconds:       int64(age.Seconds()),
		ThresholdSeconds: int64(freshMax.Seconds()),
	}
}
