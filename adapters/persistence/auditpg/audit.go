// Package auditpg implements the audit module's Store port on PostgreSQL.
// Audit rows are append-only (immutability trigger); the insert joins the
// caller's transaction so an audit failure fails the whole command (audit is
// mandatory, not best-effort — module architecture §3.8).
package auditpg

import (
	"context"
	"fmt"

	"github.com/forecastiq/forecastiq/internal/audit"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
	"github.com/forecastiq/forecastiq/internal/platform/ids"
)

// Store implements audit.Store.
type Store struct{}

// NewStore returns an audit Store.
func NewStore() *Store { return &Store{} }

// Insert implements audit.Store.
func (s *Store) Insert(ctx context.Context, tx dbtx.DBTX, e audit.Event) error {
	var ipAddress any
	if e.IPAddress != "" {
		ipAddress = e.IPAddress
	}
	details := e.Details
	if details == nil {
		details = map[string]any{}
	}
	_, err := tx.Exec(ctx,
		`INSERT INTO audit_events (id, user_id, action, resource_type, resource_id, details, ip_address, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		ids.New(), e.UserID, e.Action, e.ResourceType, e.ResourceID, details, ipAddress, e.At)
	if err != nil {
		return fmt.Errorf("insert audit event: %w", err)
	}
	return nil
}
