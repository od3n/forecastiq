package respond

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestComputeFreshness_States(t *testing.T) {
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	fresh := 75 * time.Minute // rankings fresh→delayed boundary example (4500 s)
	stale := 24 * time.Hour   // delayed→stale boundary

	cases := []struct {
		name  string
		age   time.Duration
		state string
	}{
		{"fresh_at_zero", 0, FreshnessFresh},
		{"fresh_at_boundary", fresh, FreshnessFresh},
		{"delayed_just_past_fresh", fresh + time.Second, FreshnessDelayed},
		{"delayed_at_boundary", stale, FreshnessDelayed},
		{"stale_past_boundary", stale + time.Second, FreshnessStale},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lu := now.Add(-tc.age)
			f := ComputeFreshness(&lu, now, fresh, stale, "")
			assert.Equal(t, tc.state, f.State)
			assert.Equal(t, int64(fresh.Seconds()), f.ThresholdSeconds)
			assert.EqualValues(t, int64(tc.age.Seconds()), f.AgeSeconds)
			assert.NotNil(t, f.LastUpdated)
		})
	}
}

func TestComputeFreshness_Unavailable(t *testing.T) {
	now := time.Now().UTC()
	f := ComputeFreshness(nil, now, time.Hour, 2*time.Hour, "circuit_open")
	assert.Equal(t, FreshnessUnavailable, f.State)
	assert.Equal(t, "circuit_open", f.Reason)
	assert.Nil(t, f.LastUpdated)
}

func TestComputeFreshness_FutureClamped(t *testing.T) {
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour) // clock skew: datum "after" now
	f := ComputeFreshness(&future, now, time.Hour, 2*time.Hour, "")
	assert.Equal(t, FreshnessFresh, f.State)
	assert.EqualValues(t, 0, f.AgeSeconds)
}
