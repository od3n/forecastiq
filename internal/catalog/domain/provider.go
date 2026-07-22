package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Provider is the catalog aggregate root for a weather forecast source.
// Attribution fields are mandatory (BR-ATTR-01) and surfaced on every
// provider-derived payload.
type Provider struct {
	ID              uuid.UUID
	Name            string
	Slug            string
	APIBaseURL      string
	Status          Status
	AttributionText string
	AttributionURL  string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Validate checks provider invariants.
func (p *Provider) Validate() error {
	ve := &ValidationError{}
	if strings.TrimSpace(p.Name) == "" {
		ve.Add("name", "must not be empty")
	}
	if strings.TrimSpace(p.Slug) == "" {
		ve.Add("slug", "must not be empty")
	}
	if strings.TrimSpace(p.APIBaseURL) == "" {
		ve.Add("api_base_url", "must not be empty")
	}
	if strings.TrimSpace(p.AttributionText) == "" {
		ve.Add("attribution_text", "must not be empty (BR-ATTR-01)")
	}
	if strings.TrimSpace(p.AttributionURL) == "" {
		ve.Add("attribution_url", "must not be empty (BR-ATTR-01)")
	}
	if !p.Status.Valid() {
		ve.Add("status", "must be one of active|disabled|archived")
	}
	return ve.ErrorOrNil()
}
