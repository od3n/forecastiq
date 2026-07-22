package ratelimit_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/forecastiq/forecastiq/internal/platform/clock"
	"github.com/forecastiq/forecastiq/internal/platform/ratelimit"
)

func TestLimiter_Allow(t *testing.T) {
	clk := clock.NewMutable(time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC))
	l := ratelimit.NewLimiter(2, 1, clk) // burst 2, 1/sec

	assert.True(t, l.Allow())
	assert.True(t, l.Allow())
	assert.False(t, l.Allow()) // exhausted

	clk.Advance(time.Second) // refill 1 token
	assert.True(t, l.Allow())
	assert.False(t, l.Allow())
}

func TestLimiter_Wait(t *testing.T) {
	// Real clock: refill progresses with wall time so Wait can acquire.
	l := ratelimit.NewLimiter(1, 100, nil) // 1 burst, 100/sec
	require.True(t, l.Allow())

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	assert.NoError(t, l.Wait(ctx))
}

func TestLimiter_Wait_ContextCancelled(t *testing.T) {
	clk := clock.NewMutable(time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC))
	l := ratelimit.NewLimiter(1, 0.001, clk) // very slow refill
	require.True(t, l.Allow())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	assert.Error(t, l.Wait(ctx))
}

func TestKeyedLimiter_IndependentBuckets(t *testing.T) {
	clk := clock.NewMutable(time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC))
	kl := ratelimit.NewKeyedLimiter(1, 0.001, clk)

	assert.True(t, kl.Allow("a"))
	assert.False(t, kl.Allow("a"))
	assert.True(t, kl.Allow("b")) // separate bucket
}
