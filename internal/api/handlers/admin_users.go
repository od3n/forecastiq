package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/api/respond"
	"github.com/forecastiq/forecastiq/internal/identity"
)

// SetUserStatusRequest is the PATCH /admin/users/{id}/status body.
type SetUserStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

// SetUserRoleRequest is the PATCH /admin/users/{id}/role body.
type SetUserRoleRequest struct {
	Role string `json:"role" binding:"required"`
}

// principalActor rebuilds the identity principal from the request principal for
// the identity use cases (which own the self-lockout + audit-actor logic).
func principalActor(c *gin.Context) (identity.Principal, bool) {
	p, ok := respond.PrincipalFrom(c)
	if !ok || p.UserID == nil {
		return identity.Principal{}, false
	}
	ap := identity.Principal{UserID: *p.UserID, Email: p.Email, Role: identity.Role(p.Role)}
	if p.WorkspaceID != nil {
		ap.WorkspaceID = *p.WorkspaceID
	}
	return ap, true
}

// ListUsers godoc
// @Summary      List users (S-14, admin)
// @Description  Keyset-paginated user list (newest first). Admin only; no-store.
// @Tags         admin
// @Produce      json
// @Param        cursor query string false "pagination cursor (last id)"
// @Param        limit  query int    false "page size (1..200)" default(50)
// @Success      200 {object} respond.Envelope
// @Failure      401 {object} respond.Problem
// @Failure      403 {object} respond.Problem
// @Router       /admin/users [get]
func (h *Handlers) ListUsers(c *gin.Context) {
	limit := 50
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	var cursor uuid.UUID
	if v := c.Query("cursor"); v != "" {
		cur, err := parseUUIDParam(v, "cursor")
		if err != nil {
			respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
			return
		}
		cursor = cur
	}
	users, err := h.UserAdmin.List(c.Request.Context(), limit, cursor)
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	dtos := make([]userDTO, 0, len(users))
	for _, u := range users {
		dtos = append(dtos, userToDTO(u))
	}
	c.Header("Cache-Control", "no-store")
	opts := respond.Options{RequestID: respond.RequestID(c)}
	if len(dtos) == limit && limit > 0 {
		opts.Pagination = &respond.Pagination{NextCursor: dtos[len(dtos)-1].ID, HasMore: true}
	}
	respond.OK(c, gin.H{"users": dtos}, opts)
}

// SetUserStatus godoc
// @Summary      Disable/enable a user (S-14, admin)
// @Description  Sets a user's status (active|disabled). Refuses self-target
// @Description  (409 self-lockout). Bans/unbans in the auth provider first, then
// @Description  persists + audits. Admin only; no-store.
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        id   path string true "user id (UUID)"
// @Param        body body handlers.SetUserStatusRequest true "status"
// @Success      200 {object} respond.Envelope
// @Failure      401 {object} respond.Problem
// @Failure      403 {object} respond.Problem
// @Failure      409 {object} respond.Problem
// @Failure      422 {object} respond.Problem
// @Router       /admin/users/{id}/status [patch]
func (h *Handlers) SetUserStatus(c *gin.Context) {
	actor, ok := principalActor(c)
	if !ok {
		respond.Error(c, respond.ErrUnauthorized, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	var req SetUserStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Error(c, &fieldErr{field: "status", message: "status is required"}, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	user, err := h.UserAdmin.SetStatus(c.Request.Context(), actor, id, req.Status, c.ClientIP())
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	c.Header("Cache-Control", "no-store")
	respond.OK(c, gin.H{"user": userToDTO(user)}, respond.Options{RequestID: respond.RequestID(c)})
}

// SetUserRole godoc
// @Summary      Change a user's role (S-14, admin)
// @Description  Sets a user's application role (user|admin). Refuses self-target
// @Description  (409 self-lockout). The database is the authoritative role
// @Description  source, so the change takes effect on the target's next request.
// @Description  Audited. Admin only; no-store.
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        id   path string true "user id (UUID)"
// @Param        body body handlers.SetUserRoleRequest true "role"
// @Success      200 {object} respond.Envelope
// @Failure      401 {object} respond.Problem
// @Failure      403 {object} respond.Problem
// @Failure      404 {object} respond.Problem
// @Failure      409 {object} respond.Problem
// @Failure      422 {object} respond.Problem
// @Router       /admin/users/{id}/role [patch]
func (h *Handlers) SetUserRole(c *gin.Context) {
	actor, ok := principalActor(c)
	if !ok {
		respond.Error(c, respond.ErrUnauthorized, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	var req SetUserRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Error(c, &fieldErr{field: "role", message: "role is required"}, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	user, err := h.UserAdmin.SetRole(c.Request.Context(), actor, id, req.Role, c.ClientIP())
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	c.Header("Cache-Control", "no-store")
	respond.OK(c, gin.H{"user": userToDTO(user)}, respond.Options{RequestID: respond.RequestID(c)})
}

// DeleteUser godoc
// @Summary      Delete a user (S-14, admin; AUTH-09)
// @Description  Deletes a user account. Refuses self-target (409 self-lockout).
// @Description  Local rows removed (keys cascade; audit anonymized), then the
// @Description  auth provider delete propagates. Admin only.
// @Tags         admin
// @Param        id path string true "user id (UUID)"
// @Success      204 "deleted"
// @Failure      401 {object} respond.Problem
// @Failure      403 {object} respond.Problem
// @Failure      404 {object} respond.Problem
// @Failure      409 {object} respond.Problem
// @Router       /admin/users/{id} [delete]
func (h *Handlers) DeleteUser(c *gin.Context) {
	actor, ok := principalActor(c)
	if !ok {
		respond.Error(c, respond.ErrUnauthorized, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	if err := h.UserAdmin.Delete(c.Request.Context(), actor, id, false, c.ClientIP()); err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Status(http.StatusNoContent)
}

// DeleteMe godoc
// @Summary      Delete own account (S-09; AUTH-09)
// @Description  Self-service account deletion. Local rows removed (keys cascade;
// @Description  audit anonymized), then the auth provider delete propagates.
// @Tags         identity
// @Success      204 "deleted"
// @Failure      401 {object} respond.Problem
// @Router       /me [delete]
func (h *Handlers) DeleteMe(c *gin.Context) {
	actor, ok := principalActor(c)
	if !ok {
		respond.Error(c, respond.ErrUnauthorized, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	if err := h.UserAdmin.Delete(c.Request.Context(), actor, actor.UserID, true, c.ClientIP()); err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Status(http.StatusNoContent)
}
