// Package perfids centralizes the deterministic catalog identifiers of the
// WP-26 performance dataset so the seeder, load harnesses, and query-baseline
// tools address the same rows without sharing state.
package perfids

import (
	"fmt"

	"github.com/google/uuid"
)

// LocationID returns the deterministic id of perf location i
// (00000000-0000-0000-0001-…). Distinct from the canonical seed range
// (…-0000-00000000003x) so perf rows never collide with operator data.
func LocationID(i int) uuid.UUID {
	return uuid.MustParse(fmt.Sprintf("00000000-0000-0000-0001-%012x", i))
}

// ProviderID / ConfigID cover synthetic providers beyond the two canonical
// ones (index ≥ 2).
func ProviderID(i int) uuid.UUID {
	return uuid.MustParse(fmt.Sprintf("00000000-0000-0000-0002-%012x", i))
}

// ConfigID returns the provider-configuration id for synthetic provider i.
func ConfigID(i int) uuid.UUID {
	return uuid.MustParse(fmt.Sprintf("00000000-0000-0000-0003-%012x", i))
}

// AdminUserID is the perf admin user backing ADMIN_TOKEN=perf-admin-token in
// dev-verifier environments.
var AdminUserID = uuid.MustParse("00000000-0000-0000-0004-000000000001")

// CanonicalHorizons are the UI horizon options (doc 02 §3.1) used for the
// pre-aggregated metric/ranking backfill and the query baselines.
var CanonicalHorizons = []int{60, 180, 360, 720, 1440, 4320, 10080}
