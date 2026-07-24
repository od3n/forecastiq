package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/api/respond"
	"github.com/forecastiq/forecastiq/internal/identity"
)

// userDTO is the self-service profile representation (S-09). It carries no auth
// material: the Supabase subject stays internal, and role/status are the
// database-authoritative values (never a token claim).
type userDTO struct {
	ID                string         `json:"id"`
	Email             string         `json:"email"`
	Role              string         `json:"role"`
	Status            string         `json:"status"`
	DefaultLocationID *string        `json:"default_location_id,omitempty"`
	Preferences       map[string]any `json:"preferences,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
	LastLoginAt       *time.Time     `json:"last_login_at,omitempty"`
}

func userToDTO(u *identity.User) userDTO {
	dto := userDTO{
		ID: u.ID.String(), Email: u.Email, Role: string(u.Role), Status: u.Status,
		Preferences: u.Preferences, CreatedAt: u.CreatedAt.UTC(), LastLoginAt: u.LastLoginAt,
	}
	if u.DefaultLocationID != nil {
		s := u.DefaultLocationID.String()
		dto.DefaultLocationID = &s
	}
	return dto
}

// apiKeyDTO is a personal API key's metadata. The secret and its hash are never
// serialized (only shown once, at creation, via the CreateAPIKey response).
type apiKeyDTO struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	KeyPrefix       string     `json:"key_prefix"`
	Scopes          []string   `json:"scopes"`
	RateLimitPerMin int        `json:"rate_limit_per_min"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	RevokedAt       *time.Time `json:"revoked_at,omitempty"`
	LastUsedAt      *time.Time `json:"last_used_at,omitempty"`
}

func apiKeyToDTO(k *identity.APIKey) apiKeyDTO {
	return apiKeyDTO{
		ID: k.ID.String(), Name: k.Name, KeyPrefix: k.KeyPrefix, Scopes: k.Scopes,
		RateLimitPerMin: k.RateLimitPerMin, ExpiresAt: k.ExpiresAt,
		CreatedAt: k.CreatedAt.UTC(), RevokedAt: k.RevokedAt, LastUsedAt: k.LastUsedAt,
	}
}

// UpdateProfileRequest is the PATCH /me body (mutable fields only; nil = leave
// unchanged). Identity fields (email, role, status) are never accepted here.
type UpdateProfileRequest struct {
	DefaultLocationID *string        `json:"default_location_id"`
	Preferences       map[string]any `json:"preferences"`
}

// CreateAPIKeyRequest is the POST /api-keys body.
type CreateAPIKeyRequest struct {
	Name            string     `json:"name" binding:"required"`
	Scopes          []string   `json:"scopes"`
	RateLimitPerMin int        `json:"rate_limit_per_min"`
	ExpiresAt       *time.Time `json:"expires_at"`
}

// currentUserID returns the authenticated principal's user id. RequireAuth
// guarantees a populated principal; a defensive nil-guard yields 401.
func currentUserID(c *gin.Context) (uuid.UUID, bool) {
	p, ok := respond.PrincipalFrom(c)
	if !ok || p.UserID == nil {
		respond.Error(c, respond.ErrUnauthorized, respond.RequestID(c), c.Request.URL.Path)
		return uuid.Nil, false
	}
	return *p.UserID, true
}

// GetMe godoc
// @Summary      Current user profile (S-09)
// @Description  Returns the authenticated user's profile. Self-scope; no-store.
// @Tags         identity
// @Produce      json
// @Success      200 {object} respond.Envelope
// @Failure      401 {object} respond.Problem
// @Router       /me [get]
func (h *Handlers) GetMe(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		return
	}
	u, err := h.Users.GetMe(c.Request.Context(), uid)
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	c.Header("Cache-Control", "no-store")
	respond.OK(c, gin.H{"user": userToDTO(u)}, respond.Options{RequestID: respond.RequestID(c)})
}

// UpdateMe godoc
// @Summary      Update current user profile (S-09)
// @Description  Updates the authenticated user's mutable fields (default
// @Description  location, preferences). Self-scope; audited; no-store.
// @Tags         identity
// @Accept       json
// @Produce      json
// @Param        body body handlers.UpdateProfileRequest true "profile fields"
// @Success      200 {object} respond.Envelope
// @Failure      401 {object} respond.Problem
// @Failure      422 {object} respond.Problem
// @Router       /me [patch]
func (h *Handlers) UpdateMe(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		return
	}
	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Error(c, &fieldErr{field: "body", message: "invalid JSON body"}, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	in := identity.UpdateProfileInput{Preferences: req.Preferences, Actor: identity.Actor{IPAddress: c.ClientIP()}}
	if req.DefaultLocationID != nil {
		locID, err := parseUUIDParam(*req.DefaultLocationID, "default_location_id")
		if err != nil {
			respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
			return
		}
		in.DefaultLocationID = &locID
	}
	u, err := h.Users.UpdateMe(c.Request.Context(), uid, in)
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	c.Header("Cache-Control", "no-store")
	respond.OK(c, gin.H{"user": userToDTO(u)}, respond.Options{RequestID: respond.RequestID(c)})
}

// ListAPIKeys godoc
// @Summary      List personal API keys (S-09)
// @Description  Lists the authenticated user's API keys (metadata only; never
// @Description  the secret). Self-scope; no-store.
// @Tags         identity
// @Produce      json
// @Success      200 {object} respond.Envelope
// @Failure      401 {object} respond.Problem
// @Router       /api-keys [get]
func (h *Handlers) ListAPIKeys(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		return
	}
	keys, err := h.Keys.ListKeys(c.Request.Context(), uid)
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	dtos := make([]apiKeyDTO, 0, len(keys))
	for _, k := range keys {
		dtos = append(dtos, apiKeyToDTO(k))
	}
	c.Header("Cache-Control", "no-store")
	respond.OK(c, gin.H{"api_keys": dtos}, respond.Options{RequestID: respond.RequestID(c)})
}

// CreateAPIKey godoc
// @Summary      Create a personal API key (S-09)
// @Description  Mints an API key for the authenticated user. The plaintext key
// @Description  is returned exactly once and never re-derivable. Audited; no-store.
// @Tags         identity
// @Accept       json
// @Produce      json
// @Param        body body handlers.CreateAPIKeyRequest true "key request"
// @Success      201 {object} respond.Envelope
// @Failure      401 {object} respond.Problem
// @Failure      422 {object} respond.Problem
// @Router       /api-keys [post]
func (h *Handlers) CreateAPIKey(c *gin.Context) {
	p, ok := respond.PrincipalFrom(c)
	if !ok || p.UserID == nil || p.WorkspaceID == nil {
		respond.Error(c, respond.ErrUnauthorized, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	var req CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Error(c, &fieldErr{field: "name", message: "name is required"}, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	in := identity.CreateKeyInput{
		Name: req.Name, Scopes: req.Scopes, RateLimitPerMin: req.RateLimitPerMin,
		ExpiresAt: req.ExpiresAt, Actor: identity.Actor{IPAddress: c.ClientIP()},
	}
	created, err := h.Keys.CreateKey(c.Request.Context(),
		identity.Principal{UserID: *p.UserID, WorkspaceID: *p.WorkspaceID}, in)
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	c.Header("Cache-Control", "no-store")
	respond.Created(c, gin.H{
		"api_key": apiKeyToDTO(created.Key),
		"key":     created.Plaintext, // shown once; the value for the X-API-Key header
	}, respond.Options{RequestID: respond.RequestID(c)})
}

// RevokeAPIKey godoc
// @Summary      Revoke a personal API key (S-09)
// @Description  Revokes a key the authenticated user owns (idempotent). Unknown
// @Description  or non-owned keys return 404 (no existence disclosure). Audited.
// @Tags         identity
// @Param        id path string true "API key id (UUID)"
// @Success      204 "revoked"
// @Failure      401 {object} respond.Problem
// @Failure      404 {object} respond.Problem
// @Router       /api-keys/{id} [delete]
func (h *Handlers) RevokeAPIKey(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		return
	}
	keyID, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	if err := h.Keys.RevokeKey(c.Request.Context(), uid, keyID, identity.Actor{IPAddress: c.ClientIP()}); err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Status(http.StatusNoContent)
}
