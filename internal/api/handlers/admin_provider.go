package handlers

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/forecastiq/forecastiq/internal/api/respond"
	"github.com/forecastiq/forecastiq/internal/catalog"
)

// ── Request bodies ────────────────────────────────────────────────────

// SetProviderStatusRequest is the PATCH /admin/providers/{id}/status body.
type SetProviderStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

// UpdateProviderConfigRequest is the PATCH /admin/provider-configurations/{id}
// body. Only operator-mutable fields; the credential reference is never
// accepted or returned (BR-08; credential never echoed).
type UpdateProviderConfigRequest struct {
	Status          *string `json:"status"`
	MinuteOffset    *int    `json:"minute_offset"`
	AdapterVersion  *string `json:"adapter_version"`
	ValidationState *string `json:"validation_state"`
}

// ── DTOs ──────────────────────────────────────────────────────────────

type scheduleDTO struct {
	Interval     string `json:"interval"`
	MinuteOffset int    `json:"minute_offset"`
}

// providerConfigDTO is the admin configuration view. It exposes has_credential
// (a boolean) rather than the credential reference — the credential is never
// echoed (S-11).
type providerConfigDTO struct {
	ID                 string      `json:"id"`
	ProviderID         string      `json:"provider_id"`
	WorkspaceID        string      `json:"workspace_id"`
	Status             string      `json:"status"`
	HasCredential      bool        `json:"has_credential"`
	CollectionSchedule scheduleDTO `json:"collection_schedule"`
	AdapterVersion     string      `json:"adapter_version"`
	ValidationState    string      `json:"validation_state"`
	UpdatedAt          time.Time   `json:"updated_at"`
}

func providerConfigDTOOf(c *catalog.ProviderConfiguration) providerConfigDTO {
	return providerConfigDTO{
		ID: c.ID.String(), ProviderID: c.ProviderID.String(), WorkspaceID: c.WorkspaceID.String(),
		Status: string(c.Status), HasCredential: c.CredentialRef != "",
		CollectionSchedule: scheduleDTO{Interval: c.CollectionSchedule.Interval, MinuteOffset: c.CollectionSchedule.MinuteOffset},
		AdapterVersion:     c.AdapterVersion, ValidationState: c.ValidationState, UpdatedAt: c.UpdatedAt.UTC(),
	}
}

// SetProviderStatus godoc
// @Summary      Enable or disable a provider (S-11, admin)
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        id path string true "provider id"
// @Param        body body SetProviderStatusRequest true "status"
// @Success      200 {object} respond.Envelope
// @Failure      401 {object} respond.Problem
// @Failure      404 {object} respond.Problem
// @Failure      422 {object} respond.Problem
// @Router       /admin/providers/{id}/status [patch]
func (h *Handlers) SetProviderStatus(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	var req SetProviderStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Error(c, &badField{detail: err.Error()}, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	p, err := h.ProviderAdmin.SetProviderStatus(c.Request.Context(), id, catalog.Status(req.Status), actor(c))
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	respond.OK(c, providerDTO(p), respond.Options{RequestID: respond.RequestID(c)})
}

// UpdateProviderConfiguration godoc
// @Summary      Update a provider configuration (S-11, admin)
// @Description  Updates operator-mutable fields (status, collection minute
// @Description  offset, adapter version, validation state). The credential
// @Description  reference is never accepted or returned.
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        id path string true "configuration id"
// @Param        body body UpdateProviderConfigRequest true "config"
// @Success      200 {object} respond.Envelope
// @Failure      401 {object} respond.Problem
// @Failure      404 {object} respond.Problem
// @Failure      422 {object} respond.Problem
// @Router       /admin/provider-configurations/{id} [patch]
func (h *Handlers) UpdateProviderConfiguration(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	var req UpdateProviderConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Error(c, &badField{detail: err.Error()}, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	in := catalog.UpdateConfigInput{MinuteOffset: req.MinuteOffset, AdapterVersion: req.AdapterVersion, ValidationState: req.ValidationState}
	if req.Status != nil {
		st := catalog.Status(*req.Status)
		in.Status = &st
	}
	cfg, err := h.ConfigAdmin.UpdateConfiguration(c.Request.Context(), id, in, actor(c))
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	respond.OK(c, providerConfigDTOOf(cfg), respond.Options{RequestID: respond.RequestID(c)})
}
