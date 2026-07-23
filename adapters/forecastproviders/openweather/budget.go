package openweather

import (
	"sync"
	"time"

	"github.com/forecastiq/forecastiq/internal/platform/clock"
)

// dailyBudget is a UTC-day request budget with 429-triggered pause. It is the
// WP-07 outbound rate-budget guard: it caps upstream calls per UTC day and,
// once the provider signals 429, pauses every further call until the advised
// retry time (or, absent a Retry-After hint, until the next UTC midnight). The
// counter rolls over at the UTC day boundary. All methods are safe for
// concurrent use (the scheduler dispatches collections in parallel).
type dailyBudget struct {
	mu        sync.Mutex
	max       int
	clock     clock.Clock
	day       time.Time // UTC midnight the counter currently belongs to
	used      int
	pausedTil time.Time // zero value = not paused
}

// newDailyBudget returns a budget allowing max calls per UTC day.
func newDailyBudget(max int, clk clock.Clock) *dailyBudget {
	if clk == nil {
		clk = clock.Real{}
	}
	return &dailyBudget{max: max, clock: clk, day: utcDay(clk.Now())}
}

// reserve reports whether a call may proceed at now. It rolls the counter over
// at the UTC day boundary, clears an elapsed pause, then checks the pause and
// the remaining budget. On success it consumes one unit and returns (true, 0);
// when blocked it returns (false, retryAfter) where retryAfter is the time
// until the budget frees (pause end or next UTC midnight, whichever governs).
func (b *dailyBudget) reserve(now time.Time) (bool, time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rollover(now)

	if !b.pausedTil.IsZero() {
		if now.Before(b.pausedTil) {
			return false, b.pausedTil.Sub(now)
		}
		b.pausedTil = time.Time{} // pause elapsed
	}
	if b.used >= b.max {
		return false, untilNextUTCDay(now)
	}
	b.used++
	return true, 0
}

// pause engages a pause after an upstream 429. A positive d (from Retry-After)
// pauses for that duration and lets collection resume once it elapses; a
// non-positive d pauses until the next UTC day (the free tier's 429 signals a
// spent daily allowance, so the conservative default is to rest until reset).
func (b *dailyBudget) pause(now time.Time, d time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rollover(now)
	until := now.Add(untilNextUTCDay(now))
	if d > 0 {
		until = now.Add(d)
	}
	if until.After(b.pausedTil) {
		b.pausedTil = until
	}
}

// rollover resets the counter and clears the pause when now crosses into a new
// UTC day. Caller holds the lock.
func (b *dailyBudget) rollover(now time.Time) {
	if today := utcDay(now); today.After(b.day) {
		b.day = today
		b.used = 0
		b.pausedTil = time.Time{}
	}
}

// utcDay returns the UTC midnight that starts t's day.
func utcDay(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

// untilNextUTCDay returns the duration from now to the next UTC midnight.
func untilNextUTCDay(now time.Time) time.Duration {
	return utcDay(now).Add(24 * time.Hour).Sub(now.UTC())
}
