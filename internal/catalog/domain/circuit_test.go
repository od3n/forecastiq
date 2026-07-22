package domain_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/forecastiq/forecastiq/internal/catalog/domain"
)

func TestCircuit_OpensAfterThreshold(t *testing.T) {
	c := domain.NewProviderCircuit(uuid.New())
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)

	for i := 0; i < domain.CircuitFailureThreshold-1; i++ {
		c.ApplyFailure(now)
		assert.Equal(t, domain.CircuitClosed, c.State, "should stay closed before threshold")
	}
	c.ApplyFailure(now)
	assert.Equal(t, domain.CircuitOpen, c.State)
	assert.Equal(t, domain.CircuitFailureThreshold, c.ConsecutiveFailures)
	assert.NotNil(t, c.NextProbeAt)
}

func TestCircuit_BlocksWhileOpen(t *testing.T) {
	c := domain.NewProviderCircuit(uuid.New())
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	for i := 0; i < domain.CircuitFailureThreshold; i++ {
		c.ApplyFailure(now)
	}
	// Before the probe time: blocked.
	d := c.Evaluate(now.Add(10 * time.Second))
	assert.False(t, d.Allowed)
	assert.Equal(t, domain.CircuitOpen, d.State)
}

func TestCircuit_HalfOpenProbeAfterDelay(t *testing.T) {
	c := domain.NewProviderCircuit(uuid.New())
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	for i := 0; i < domain.CircuitFailureThreshold; i++ {
		c.ApplyFailure(now)
	}
	// After the probe delay: a probe is allowed and the breaker goes half-open.
	d := c.Evaluate(now.Add(domain.CircuitProbeDelay + time.Second))
	assert.True(t, d.Allowed)
	assert.True(t, d.Probe)
	assert.Equal(t, domain.CircuitHalfOpen, c.State)
}

func TestCircuit_SuccessCloses(t *testing.T) {
	c := domain.NewProviderCircuit(uuid.New())
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	for i := 0; i < domain.CircuitFailureThreshold; i++ {
		c.ApplyFailure(now)
	}
	prev := c.ApplySuccess(now)
	assert.Equal(t, domain.CircuitOpen, prev)
	assert.Equal(t, domain.CircuitClosed, c.State)
	assert.Equal(t, 0, c.ConsecutiveFailures)
	assert.Nil(t, c.NextProbeAt)
}

func TestCircuit_GaugeValue(t *testing.T) {
	assert.Equal(t, 0.0, domain.CircuitClosed.GaugeValue())
	assert.Equal(t, 1.0, domain.CircuitHalfOpen.GaugeValue())
	assert.Equal(t, 2.0, domain.CircuitOpen.GaugeValue())
}
