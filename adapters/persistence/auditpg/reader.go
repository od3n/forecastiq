package auditpg

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/audit"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
)

// List implements audit.QueryStore: keyset pagination over the time-ordered id
// (newest first), with optional action/resource_type/user_id predicates.
// Returns up to f.Limit+1 rows so the reader can detect has_more.
func (s *Store) List(ctx context.Context, pool dbtx.DBTX, f audit.Filter) ([]audit.RecordedEvent, error) {
	query := `SELECT id, user_id, action, resource_type, resource_id, details,
	            COALESCE(host(ip_address), ''), created_at
	          FROM audit_events WHERE 1=1`
	args := []any{}
	idx := 1
	add := func(clause string, val any) {
		query += fmt.Sprintf(clause, idx)
		args = append(args, val)
		idx++
	}
	if f.Action != nil {
		add(` AND action = $%d`, *f.Action)
	}
	if f.ResourceType != nil {
		add(` AND resource_type = $%d`, *f.ResourceType)
	}
	if f.UserID != nil {
		add(` AND user_id = $%d`, *f.UserID)
	}
	if f.Cursor != uuid.Nil {
		add(` AND id < $%d`, f.Cursor)
	}
	query += fmt.Sprintf(` ORDER BY id DESC LIMIT $%d`, idx)
	args = append(args, f.Limit+1)

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list audit events: %w", err)
	}
	defer rows.Close()
	var out []audit.RecordedEvent
	for rows.Next() {
		var e audit.RecordedEvent
		var details []byte
		if serr := rows.Scan(&e.ID, &e.UserID, &e.Action, &e.ResourceType, &e.ResourceID,
			&details, &e.IPAddress, &e.CreatedAt); serr != nil {
			return nil, fmt.Errorf("scan audit event: %w", serr)
		}
		if len(details) > 0 {
			if derr := json.Unmarshal(details, &e.Details); derr != nil {
				return nil, fmt.Errorf("decode audit details: %w", derr)
			}
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
