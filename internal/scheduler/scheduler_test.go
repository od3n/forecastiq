package scheduler

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/collection/domain"
)

func TestRetryBackoff(t *testing.T) {
	// FC-08 backoff: 1, 2, 4, 8, 16s, clamped at attempt 5.
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 1 * time.Second}, // clamped up to 1
		{1, 1 * time.Second},
		{2, 2 * time.Second},
		{3, 4 * time.Second},
		{4, 8 * time.Second},
		{5, 16 * time.Second},
		{9, 16 * time.Second}, // clamped down to 5
	}
	for _, c := range cases {
		if got := retryBackoff(c.attempt); got != c.want {
			t.Errorf("retryBackoff(%d) = %v, want %v", c.attempt, got, c.want)
		}
	}
}

func TestSlotLag(t *testing.T) {
	base := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	if got := slotLag(base.Add(90*time.Second), base); got != 90*time.Second {
		t.Errorf("late claim lag = %v, want 90s", got)
	}
	if got := slotLag(base.Add(-time.Minute), base); got != 0 {
		t.Errorf("early claim lag = %v, want 0", got)
	}
	if got := slotLag(base, base); got != 0 {
		t.Errorf("on-time claim lag = %v, want 0", got)
	}
}

func TestClassifyError(t *testing.T) {
	if got := classifyError(&domain.CircuitOpenError{ProviderID: uuid.New()}); got != "circuit_open" {
		t.Errorf("circuit-open classification = %q, want circuit_open", got)
	}
	if got := classifyError(domain.ErrInactive); got != "inactive" {
		t.Errorf("inactive classification = %q, want inactive", got)
	}
	if got := classifyError(errors.New("boom")); got != "error" {
		t.Errorf("generic classification = %q, want error", got)
	}
}

func TestNewConfigDefaults(t *testing.T) {
	s := New(nil, nil, nil, nil, nil, nil, nil, nil, nil, Config{Interval: 30 * time.Second})
	if s.cfg.JobTimeout != 60*time.Second {
		t.Errorf("JobTimeout default = %v, want 60s", s.cfg.JobTimeout)
	}
	if s.cfg.DrainTimeout != 30*time.Second {
		t.Errorf("DrainTimeout default = %v, want 30s", s.cfg.DrainTimeout)
	}
	// MissedThreshold defaults to 2×Interval, floored at 2 minutes.
	if s.cfg.MissedThreshold != 2*time.Minute {
		t.Errorf("MissedThreshold default = %v, want 2m (floor)", s.cfg.MissedThreshold)
	}
	big := New(nil, nil, nil, nil, nil, nil, nil, nil, nil, Config{Interval: 5 * time.Minute})
	if big.cfg.MissedThreshold != 10*time.Minute {
		t.Errorf("MissedThreshold = %v, want 10m (2×interval)", big.cfg.MissedThreshold)
	}
}

func TestTruncateMsg(t *testing.T) {
	long := make([]byte, 600)
	for i := range long {
		long[i] = 'x'
	}
	if got := truncateMsg(string(long)); len(got) != 500 {
		t.Errorf("truncateMsg length = %d, want 500", len(got))
	}
	if got := truncateMsg("short"); got != "short" {
		t.Errorf("truncateMsg(short) = %q, want short", got)
	}
}
