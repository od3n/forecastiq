package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/forecastiq/forecastiq/internal/api/respond"
	"github.com/forecastiq/forecastiq/internal/identity"
)

// exportJobDTO is the GDPR export-job representation (S-09 / S-14). The download
// link is relative; the file is served by GET /exports/{id} for 24h.
type exportJobDTO struct {
	ID          string     `json:"id"`
	Status      string     `json:"status"`
	DownloadURL string     `json:"download_url,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

func exportJobToDTO(j *identity.ExportJob) exportJobDTO {
	dto := exportJobDTO{ID: j.ID.String(), Status: j.Status, CreatedAt: j.CreatedAt.UTC()}
	if j.Status == "completed" {
		dto.DownloadURL = "/api/v1/exports/" + j.ID.String()
	}
	if j.ExpiresAt != nil {
		e := j.ExpiresAt.UTC()
		dto.ExpiresAt = &e
	}
	return dto
}

// RequestMyExport godoc
// @Summary      Request own GDPR export (S-09; AUTH-09)
// @Description  Creates an account-data export (user row + API-key metadata +
// @Description  own audit events). One active job per user (409). Self; no-store.
// @Tags         identity
// @Produce      json
// @Success      201 {object} respond.Envelope
// @Failure      401 {object} respond.Problem
// @Failure      409 {object} respond.Problem
// @Router       /me/export [post]
func (h *Handlers) RequestMyExport(c *gin.Context) {
	actor, ok := principalActor(c)
	if !ok {
		respond.Error(c, respond.ErrUnauthorized, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	job, err := h.Exports.RequestExport(c.Request.Context(), actor, actor.UserID, c.ClientIP())
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	c.Header("Cache-Control", "no-store")
	respond.Created(c, gin.H{"export": exportJobToDTO(job)}, respond.Options{RequestID: respond.RequestID(c)})
}

// RequestUserExport godoc
// @Summary      Request a user's GDPR export (S-14, admin; AUTH-09)
// @Description  Admin-triggered account-data export for a target user. One
// @Description  active job per user (409). Admin only; no-store.
// @Tags         admin
// @Produce      json
// @Param        id path string true "target user id (UUID)"
// @Success      201 {object} respond.Envelope
// @Failure      401 {object} respond.Problem
// @Failure      403 {object} respond.Problem
// @Failure      404 {object} respond.Problem
// @Failure      409 {object} respond.Problem
// @Router       /admin/users/{id}/export [post]
func (h *Handlers) RequestUserExport(c *gin.Context) {
	actor, ok := principalActor(c)
	if !ok {
		respond.Error(c, respond.ErrUnauthorized, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	job, err := h.Exports.RequestExport(c.Request.Context(), actor, id, c.ClientIP())
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	c.Header("Cache-Control", "no-store")
	respond.Created(c, gin.H{"export": exportJobToDTO(job)}, respond.Options{RequestID: respond.RequestID(c)})
}

// DownloadExport godoc
// @Summary      Download a GDPR export (AUTH-09)
// @Description  Streams the account-data JSON for a completed, unexpired export
// @Description  the caller may access (requester, target, or admin). Self/admin;
// @Description  no-store; 404 for unknown/non-owned, 410 when the 24h window closed.
// @Tags         identity
// @Produce      json
// @Param        id path string true "export id (UUID)"
// @Success      200 {object} object
// @Failure      401 {object} respond.Problem
// @Failure      404 {object} respond.Problem
// @Failure      410 {object} respond.Problem
// @Router       /exports/{id} [get]
func (h *Handlers) DownloadExport(c *gin.Context) {
	actor, ok := principalActor(c)
	if !ok {
		respond.Error(c, respond.ErrUnauthorized, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	data, err := h.Exports.DownloadExport(c.Request.Context(), actor, id)
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="forecastiq-export-%s.json"`, id))
	c.Data(http.StatusOK, "application/json; charset=utf-8", data)
}
