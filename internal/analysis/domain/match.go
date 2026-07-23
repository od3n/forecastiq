// Package domain holds the analysis module's entities and the deterministic
// matching logic (ADR-014; docs/workflows/03-matching.md). The matching engine
// pairs a forecast snapshot with exactly one observation for its target hour
// using a total-order candidate selection so the same inputs always produce the
// same pair (permutation-invariant, property-tested).
package domain

import (
	"time"

	"github.com/google/uuid"
)

// MatchRule is how a snapshot was matched to an observation (methodology §3.1).
const (
	MatchExactHour     = "exact_hour"
	MatchSubHourly15   = "sub_hourly_15min"
	QualitySuspect     = "suspect"
	QualityCorrected   = "corrected"
	StationObservation = "station_observation"
	Interpolated       = "interpolated"
	Reanalysis         = "reanalysis"
	ProviderEstimated  = "provider_estimated"
)

// provenanceRank orders observation types (BR-MATCH-03): station best, then
// interpolated = reanalysis, then provider_estimated. Unknown types sort last
// so they never outrank a recognized source.
func provenanceRank(observationType string) int {
	switch observationType {
	case StationObservation:
		return 1
	case Interpolated, Reanalysis:
		return 2
	case ProviderEstimated:
		return 3
	default:
		return 99
	}
}

// SnapshotToMatch is the read model of an unmatched forecast snapshot the
// engine must pair (a subset of forecast_snapshots).
type SnapshotToMatch struct {
	ID                     uuid.UUID
	ProviderID             uuid.UUID
	LocationID             uuid.UUID
	TargetTime             time.Time
	ForecastHorizonMinutes int
}

// ObservationCandidate is a live (non-suspect, non-superseded) observation for
// a target hour, considered for pairing.
type ObservationCandidate struct {
	ID              uuid.UUID
	ObservationType string
	QualityFlag     string
	ObservedAt      time.Time
}

// MatchedEvaluation is one immutable snapshot–observation pair (domain §2.8).
type MatchedEvaluation struct {
	ID                     uuid.UUID
	ForecastSnapshotID     uuid.UUID
	ObservationID          uuid.UUID
	ProviderID             uuid.UUID
	LocationID             uuid.UUID
	ForecastHorizonMinutes int
	TargetTime             time.Time
	MatchRule              string
	TimeDeltaMinutes       int
	ComputedAt             time.Time
}

// SelectCandidate returns the winning observation for targetHour under the
// normative total order (workflow §2): corrected preference → provenance rank →
// nearest to the top of the hour → id. The order is total (no ties survive the
// id tiebreak), so the result is invariant to the input ordering. Returns nil
// for an empty candidate set.
func SelectCandidate(targetHour time.Time, candidates []*ObservationCandidate) *ObservationCandidate {
	var best *ObservationCandidate
	for _, c := range candidates {
		if c == nil {
			continue
		}
		if best == nil || candidateLess(c, best, targetHour) {
			best = c
		}
	}
	return best
}

// candidateLess reports whether a should be preferred over b for targetHour.
// It implements the strict total order used by SelectCandidate.
func candidateLess(a, b *ObservationCandidate, targetHour time.Time) bool {
	// 1. Corrected observations are preferred (BR-MATCH-04).
	ac, bc := a.QualityFlag == QualityCorrected, b.QualityFlag == QualityCorrected
	if ac != bc {
		return ac
	}
	// 2. Provenance rank ascending (BR-MATCH-03).
	if ra, rb := provenanceRank(a.ObservationType), provenanceRank(b.ObservationType); ra != rb {
		return ra < rb
	}
	// 3. Nearest to the top of the hour ascending.
	if da, db := absDuration(a.ObservedAt.Sub(targetHour)), absDuration(b.ObservedAt.Sub(targetHour)); da != db {
		return da < db
	}
	// 4. Final deterministic tiebreak: smallest id.
	return a.ID.String() < b.ID.String()
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
