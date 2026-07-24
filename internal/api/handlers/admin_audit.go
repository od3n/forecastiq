package handlers

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/forecastiq/forecastiq/internal/api/respond"
	"github.com/forecastiq/forecastiq/internal/audit"
)

// auditEventDTO is one audit record for the S-14 admin audit screen. Details are
// pre-sanitized at write time (never credentials/payloads; audit recorder doc).
type auditEventDTO struct {
	ID           string         `json:"id"`
	UserID       *string        `json:"user_id,omitempty"`
	Action       string         `json:"action"`
	ResourceType string         `json:"resource_type"`
	ResourceID   *string        `json:"resource_id,omitempty"`
	Details      map[string]any `json:"details,omitempty"`
	IPAddress    string         `json:"ip_address,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
}

// AuditEvents godoc
// @Summary      Audit events (S-14, admin)
// @Description  Keyset-paginated audit trail (newest first), filterable by
// @Description  action, resource_type, and user_id. Admin only; read (not audited).
// @Tags         admin
// @Produce      json
// @Param        action        query string false "filter by action"
// @Param        resource_type query string false "filter by resource type"
// @Param        user_id       query string false "filter by acting user id (UUID)"
// @Param        cursor        query string false "pagination cursor (last id)"
// @Param        limit         query int    false "page size (1..200)" default(50)
// @Success      200 {object} respond.Envelope
// @Failure      401 {object} respond.Problem
// @Failure      403 {object} respond.Problem
// @Router       /admin/audit-events [get]
func (h *Handlers) AuditEvents(c *gin.Context) {
	f := audit.Filter{Limit: 50}
	if v := c.Query("action"); v != "" {
		f.Action = &v
	}
	if v := c.Query("resource_type"); v != "" {
		f.ResourceType = &v
	}
	if uid, ok := queryUUID(c, "user_id"); !ok {
		return
	} else if uid != nil {
		f.UserID = uid
	}
	if v := c.Query("cursor"); v != "" {
		cur, err := parseUUIDParam(v, "cursor")
		if err != nil {
			respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
			return
		}
		f.Cursor = cur
	}
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			f.Limit = n
		}
	}

	events, page, err := h.Audit.List(c.Request.Context(), f)
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	dtos := make([]auditEventDTO, 0, len(events))
	for _, e := range events {
		dto := auditEventDTO{
			ID: e.ID.String(), Action: e.Action, ResourceType: e.ResourceType,
			Details: e.Details, IPAddress: e.IPAddress, CreatedAt: e.CreatedAt.UTC(),
		}
		if e.UserID != nil {
			s := e.UserID.String()
			dto.UserID = &s
		}
		if e.ResourceID != nil {
			s := e.ResourceID.String()
			dto.ResourceID = &s
		}
		dtos = append(dtos, dto)
	}

	c.Header("Cache-Control", "no-store")
	respond.OK(c, gin.H{"audit_events": dtos}, respond.Options{
		RequestID:  respond.RequestID(c),
		Pagination: &respond.Pagination{NextCursor: page.NextCursor, HasMore: page.HasMore},
	})
}
