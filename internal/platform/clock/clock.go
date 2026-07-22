// Package clock provides a time source abstraction so services and domain
// logic remain deterministic under test (the clock is an injected port per
// the module architecture dependency rule).
package clock

import (
	"sync"
	"time"
)

// Clock is the time source used across the application. Production wires
// Real; tests inject Fixed or Mutable for determinism.
type Clock interface {
	Now() time.Time
}

// Real returns the actual current time (UTC).
type Real struct{}

// Now implements Clock.
func (Real) Now() time.Time { return time.Now().UTC() }

// Fixed is a Clock frozen at a single instant (test helper).
type Fixed struct{ T time.Time }

// Now implements Clock.
func (f Fixed) Now() time.Time { return f.T.UTC() }

// Mutable is a Clock whose instant can be advanced (test helper).
type Mutable struct {
	mu sync.Mutex
	t  time.Time
}

// NewMutable returns a Mutable clock set to t.
func NewMutable(t time.Time) *Mutable { return &Mutable{t: t.UTC()} }

// Now implements Clock.
func (m *Mutable) Now() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.t
}

// Advance moves the clock forward by d.
func (m *Mutable) Advance(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.t = m.t.Add(d)
}

// Set replaces the current instant.
func (m *Mutable) Set(t time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.t = t.UTC()
}
