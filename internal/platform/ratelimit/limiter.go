// Package ratelimit provides an in-process token bucket (security
// architecture §4: per-key/per-IP API limiting; workflow §4: provider
// outbound budgets). Redis-backed shared limiting is a documented promotion
// for multi-instance deployments.
package ratelimit

import (
	"context"
	"sync"
	"time"

	"github.com/forecastiq/forecastiq/internal/platform/clock"
)

// Limiter is a thread-safe token bucket.
type Limiter struct {
	mu           sync.Mutex
	tokens       float64
	max          float64
	refillPerSec float64
	last         time.Time
	clock        clock.Clock
}

// NewLimiter returns a bucket holding up to max tokens, refilling at
// refillPerSec. It starts full.
func NewLimiter(max, refillPerSec float64, clk clock.Clock) *Limiter {
	if clk == nil {
		clk = clock.Real{}
	}
	return &Limiter{tokens: max, max: max, refillPerSec: refillPerSec, last: clk.Now(), clock: clk}
}

func (l *Limiter) refill() {
	now := l.clock.Now()
	elapsed := now.Sub(l.last).Seconds()
	if elapsed > 0 {
		l.tokens += elapsed * l.refillPerSec
		if l.tokens > l.max {
			l.tokens = l.max
		}
		l.last = now
	}
}

// Allow consumes one token, reporting whether it was available.
func (l *Limiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.refill()
	if l.tokens >= 1 {
		l.tokens--
		return true
	}
	return false
}

// Wait blocks until a token is available or ctx is done. Used for provider
// outbound throttling (we delay rather than drop our own requests).
func (l *Limiter) Wait(ctx context.Context) error {
	for {
		l.mu.Lock()
		l.refill()
		if l.tokens >= 1 {
			l.tokens--
			l.mu.Unlock()
			return nil
		}
		need := 1 - l.tokens
		wait := time.Duration(need / l.refillPerSec * float64(time.Second))
		l.mu.Unlock()
		if wait < 10*time.Millisecond {
			wait = 10 * time.Millisecond
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
}

// KeyedLimiter holds one bucket per key (e.g. client IP or API key).
type KeyedLimiter struct {
	mu           sync.Mutex
	buckets      map[string]*Limiter
	max          float64
	refillPerSec float64
	clock        clock.Clock
}

// NewKeyedLimiter returns a per-key limiter; each key gets a fresh full bucket
// with the given rate.
func NewKeyedLimiter(max, refillPerSec float64, clk clock.Clock) *KeyedLimiter {
	if clk == nil {
		clk = clock.Real{}
	}
	return &KeyedLimiter{buckets: make(map[string]*Limiter), max: max, refillPerSec: refillPerSec, clock: clk}
}

// Allow consumes one token for key, creating its bucket on first use.
func (k *KeyedLimiter) Allow(key string) bool {
	k.mu.Lock()
	b, ok := k.buckets[key]
	if !ok {
		b = NewLimiter(k.max, k.refillPerSec, k.clock)
		k.buckets[key] = b
	}
	k.mu.Unlock()
	return b.Allow()
}
