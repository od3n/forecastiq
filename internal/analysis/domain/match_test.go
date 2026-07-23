package domain_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/forecastiq/forecastiq/internal/analysis/domain"
)

var hour = time.Date(2026, 7, 22, 11, 0, 0, 0, time.UTC)

func cand(id string, obsType, flag string, obsAt time.Time) *domain.ObservationCandidate {
	return &domain.ObservationCandidate{
		ID: uuid.MustParse(id), ObservationType: obsType, QualityFlag: flag, ObservedAt: obsAt,
	}
}

func TestSelectCandidate_Empty(t *testing.T) {
	assert.Nil(t, domain.SelectCandidate(hour, nil))
	assert.Nil(t, domain.SelectCandidate(hour, []*domain.ObservationCandidate{}))
}

func TestSelectCandidate_CorrectedPreferred(t *testing.T) {
	valid := cand("00000000-0000-0000-0000-000000000001", domain.StationObservation, "valid", hour)
	corrected := cand("00000000-0000-0000-0000-0000000000ff", domain.Reanalysis, domain.QualityCorrected, hour)
	// Corrected wins even though it has a worse provenance rank and a larger id.
	assert.Equal(t, corrected.ID, domain.SelectCandidate(hour, []*domain.ObservationCandidate{valid, corrected}).ID)
}

func TestSelectCandidate_ProvenanceRank(t *testing.T) {
	station := cand("00000000-0000-0000-0000-0000000000aa", domain.StationObservation, "valid", hour)
	reanalysis := cand("00000000-0000-0000-0000-000000000001", domain.Reanalysis, "valid", hour)
	// Station outranks reanalysis despite the larger id.
	assert.Equal(t, station.ID, domain.SelectCandidate(hour, []*domain.ObservationCandidate{reanalysis, station}).ID)
}

func TestSelectCandidate_NearestHourThenID(t *testing.T) {
	near := cand("00000000-0000-0000-0000-0000000000bb", domain.Reanalysis, "valid", hour.Add(2*time.Minute))
	far := cand("00000000-0000-0000-0000-000000000001", domain.Reanalysis, "valid", hour.Add(10*time.Minute))
	assert.Equal(t, near.ID, domain.SelectCandidate(hour, []*domain.ObservationCandidate{far, near}).ID)

	// Full tie on flag/rank/distance → smallest id wins.
	a := cand("00000000-0000-0000-0000-000000000001", domain.Reanalysis, "valid", hour)
	b := cand("00000000-0000-0000-0000-000000000002", domain.Reanalysis, "valid", hour)
	assert.Equal(t, a.ID, domain.SelectCandidate(hour, []*domain.ObservationCandidate{b, a}).ID)
}

// TestSelectCandidate_PermutationInvariant is the normative determinism
// property (workflow §4 / ADR-014): the chosen observation must not depend on
// the order candidates arrive in. Shuffling the input never changes the winner.
func TestSelectCandidate_PermutationInvariant(t *testing.T) {
	base := []*domain.ObservationCandidate{
		cand("00000000-0000-0000-0000-000000000010", domain.Reanalysis, "valid", hour.Add(3*time.Minute)),
		cand("00000000-0000-0000-0000-000000000020", domain.StationObservation, "valid", hour.Add(5*time.Minute)),
		cand("00000000-0000-0000-0000-000000000030", domain.Interpolated, domain.QualityCorrected, hour.Add(9*time.Minute)),
		cand("00000000-0000-0000-0000-000000000040", domain.ProviderEstimated, "valid", hour),
		cand("00000000-0000-0000-0000-000000000050", domain.StationObservation, domain.QualityCorrected, hour.Add(1*time.Minute)),
	}
	want := domain.SelectCandidate(hour, base).ID // corrected + station + nearest → id ...050

	rng := rand.New(rand.NewSource(1)) //nolint:gosec // deterministic test shuffle, not security
	for i := 0; i < 500; i++ {
		shuffled := make([]*domain.ObservationCandidate, len(base))
		copy(shuffled, base)
		rng.Shuffle(len(shuffled), func(a, b int) { shuffled[a], shuffled[b] = shuffled[b], shuffled[a] })
		got := domain.SelectCandidate(hour, shuffled)
		require.NotNil(t, got)
		assert.Equal(t, want, got.ID, "winner must be invariant to candidate order")
	}
}
