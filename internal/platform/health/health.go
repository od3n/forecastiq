// Package health provides liveness and readiness probing (observability
// architecture §4). Liveness (/healthz) is trivially 200 while the process
// runs. Readiness (/readyz) runs registered dependency checks (DB ping,
// payload-volume writability) and reports 503 until all pass — the gate
// Caddy and the deploy smoke test rely on.
package health

import (
	"context"
	"sync"
)

// Check verifies one dependency. A nil return means healthy.
type Check func(ctx context.Context) error

type namedCheck struct {
	name string
	fn   Check
}

// Checker is a registry of readiness checks.
type Checker struct {
	mu     sync.RWMutex
	checks []namedCheck
}

// NewChecker returns an empty Checker.
func NewChecker() *Checker { return &Checker{} }

// Register adds a named readiness check.
func (c *Checker) Register(name string, fn Check) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks = append(c.checks, namedCheck{name: name, fn: fn})
}

// Result is the outcome of a single check.
type Result struct {
	Name    string
	Healthy bool
	Error   string
}

// RunAll executes every check and returns per-check results plus an
// overall healthy flag (true iff every check passed).
func (c *Checker) RunAll(ctx context.Context) ([]Result, bool) {
	c.mu.RLock()
	checks := append([]namedCheck(nil), c.checks...)
	c.mu.RUnlock()

	results := make([]Result, 0, len(checks))
	allOK := true
	for _, nc := range checks {
		r := Result{Name: nc.name, Healthy: true}
		if err := nc.fn(ctx); err != nil {
			r.Healthy = false
			r.Error = err.Error()
			allOK = false
		}
		results = append(results, r)
	}
	return results, allOK
}
