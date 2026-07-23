package openweather

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/forecastiq/forecastiq/internal/platform/clock"
)

// midday is a deterministic mid-UTC-day instant used across budget tests; it is
// far from the day boundary so pause windows and the daily reset are distinct.
var midday = time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)

func TestDailyBudget_ConsumesUntilExhausted(t *testing.T) {
	clk := clock.NewMutable(midday)
	b := newDailyBudget(2, clk)

	ok, _ := b.reserve(clk.Now())
	assert.True(t, ok, "first call within budget")
	ok, _ = b.reserve(clk.Now())
	assert.True(t, ok, "second call within budget")

	ok, retryAfter := b.reserve(clk.Now())
	assert.False(t, ok, "third call exceeds the daily budget")
	// Blocked until the next UTC midnight (12:00 → 24:00 == 12h).
	assert.Equal(t, 12*time.Hour, retryAfter)
}

func TestDailyBudget_ResetsAtUTCDayBoundary(t *testing.T) {
	clk := clock.NewMutable(midday)
	b := newDailyBudget(1, clk)

	ok, _ := b.reserve(clk.Now())
	assert.True(t, ok)
	ok, _ = b.reserve(clk.Now())
	assert.False(t, ok, "budget spent for the day")

	clk.Advance(12 * time.Hour) // cross into the next UTC day
	ok, _ = b.reserve(clk.Now())
	assert.True(t, ok, "counter resets at the UTC day boundary")
}

func TestDailyBudget_PauseWithRetryAfterResumes(t *testing.T) {
	clk := clock.NewMutable(midday)
	b := newDailyBudget(100, clk) // ample budget; pause is the only constraint

	b.pause(clk.Now(), 45*time.Second)
	ok, retryAfter := b.reserve(clk.Now())
	assert.False(t, ok, "paused after 429")
	assert.Equal(t, 45*time.Second, retryAfter)

	clk.Advance(46 * time.Second)
	ok, _ = b.reserve(clk.Now())
	assert.True(t, ok, "resumes once the Retry-After window elapses")
}

func TestDailyBudget_PauseWithoutRetryAfterRestsUntilNextDay(t *testing.T) {
	clk := clock.NewMutable(midday)
	b := newDailyBudget(100, clk)

	b.pause(clk.Now(), 0) // no Retry-After hint
	ok, retryAfter := b.reserve(clk.Now())
	assert.False(t, ok)
	assert.Equal(t, 12*time.Hour, retryAfter, "rests until the next UTC midnight")

	clk.Advance(11 * time.Hour)
	ok, _ = b.reserve(clk.Now())
	assert.False(t, ok, "still paused before the day boundary")

	clk.Advance(2 * time.Hour) // now past midnight
	ok, _ = b.reserve(clk.Now())
	assert.True(t, ok, "resumes after the UTC day rolls over")
}
