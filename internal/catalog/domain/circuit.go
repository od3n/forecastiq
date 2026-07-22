package domain

import (
	"time"

	"github.com/google/uuid"
)

// Circuit breaker tuning (FC-09): open after FailureThreshold consecutive
// failures; probe (half-open) after ProbeDelay; a successful probe closes.
const (
	CircuitFailureThreshold = 5
	CircuitProbeDelay       = 60 * time.Second
)

// CircuitState is the breaker state for a provider.
type CircuitState string

const (
	CircuitClosed   CircuitState = "closed"
	CircuitHalfOpen CircuitState = "half_open"
	CircuitOpen     CircuitState = "open"
)

// GaugeValue maps the state to the circuit_state metric (0/1/2).
func (s CircuitState) GaugeValue() float64 {
	switch s {
	case CircuitHalfOpen:
		return 1
	case CircuitOpen:
		return 2
	default:
		return 0
	}
}

// ProviderCircuit is the catalog-owned persistent breaker state for a
// provider (one row per provider).
type ProviderCircuit struct {
	ProviderID          uuid.UUID
	State               CircuitState
	ConsecutiveFailures int
	LastFailureAt       *time.Time
	OpenedAt            *time.Time
	NextProbeAt         *time.Time
	UpdatedAt           time.Time
}

// NewProviderCircuit returns a closed circuit for a provider.
func NewProviderCircuit(providerID uuid.UUID) *ProviderCircuit {
	return &ProviderCircuit{ProviderID: providerID, State: CircuitClosed}
}

// Decision is the outcome of evaluating a circuit before a provider call.
type Decision struct {
	Allowed bool         // a collection attempt may proceed
	Probe   bool         // this attempt is a half-open probe
	State   CircuitState // state observed (after any probe transition)
	RetryAt *time.Time   // when open: the next probe time
}

// Evaluate decides whether a collection may proceed at `now`, applying the
// open→half-open probe transition when the probe time has arrived. The
// transition (if any) is reflected in the returned circuit so the caller can
// persist it.
func (c *ProviderCircuit) Evaluate(now time.Time) Decision {
	switch c.State {
	case CircuitOpen:
		if c.NextProbeAt != nil && !now.Before(*c.NextProbeAt) {
			c.State = CircuitHalfOpen
			return Decision{Allowed: true, Probe: true, State: CircuitHalfOpen}
		}
		return Decision{Allowed: false, State: CircuitOpen, RetryAt: c.NextProbeAt}
	case CircuitHalfOpen:
		// One probe in flight; additional concurrent attempts are held back.
		return Decision{Allowed: true, Probe: true, State: CircuitHalfOpen}
	default: // closed
		return Decision{Allowed: true, State: CircuitClosed}
	}
}

// ApplySuccess records a successful call, closing the breaker. It returns the
// previous state so the caller can detect a transition for event emission.
func (c *ProviderCircuit) ApplySuccess(now time.Time) (prev CircuitState) {
	prev = c.State
	c.State = CircuitClosed
	c.ConsecutiveFailures = 0
	c.OpenedAt = nil
	c.NextProbeAt = nil
	c.UpdatedAt = now
	return prev
}

// ApplyFailure records a failed call, opening the breaker once the failure
// threshold is reached. It returns the previous state for transition detection.
func (c *ProviderCircuit) ApplyFailure(now time.Time) (prev CircuitState) {
	prev = c.State
	c.ConsecutiveFailures++
	c.LastFailureAt = &now
	if c.ConsecutiveFailures >= CircuitFailureThreshold {
		c.State = CircuitOpen
		c.OpenedAt = &now
		probe := now.Add(CircuitProbeDelay)
		c.NextProbeAt = &probe
	}
	c.UpdatedAt = now
	return prev
}
