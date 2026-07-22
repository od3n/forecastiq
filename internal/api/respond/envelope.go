// Package respond assembles the API response envelope and RFC 7807 error
// contracts (docs/api/02-response-conventions.md, 03-error-and-partial-result-contracts.md).
// Envelope fields are included only where meaningful — never null placeholders.
package respond

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Envelope is the standard successful-response wrapper.
type Envelope struct {
	Data          any           `json:"data"`
	Metadata      *Metadata     `json:"metadata,omitempty"`
	Freshness     *Freshness    `json:"freshness,omitempty"`
	Attribution   []Attribution `json:"attribution,omitempty"`
	Warnings      []Warning     `json:"warnings,omitempty"`
	Pagination    *Pagination   `json:"pagination,omitempty"`
	PartialResult bool          `json:"partial_result,omitempty"`
}

// Metadata carries request correlation + presentation context.
type Metadata struct {
	RequestID   string            `json:"request_id"`
	GeneratedAt time.Time         `json:"generated_at"`
	Timezone    string            `json:"timezone,omitempty"`
	Units       map[string]string `json:"units,omitempty"`
}

// Freshness is the server-computed data-freshness block (BR-FRESH-02).
type Freshness struct {
	State            string     `json:"state"` // fresh | delayed | stale | unavailable
	LastUpdated      *time.Time `json:"last_updated,omitempty"`
	AgeSeconds       int64      `json:"age_seconds,omitempty"`
	ThresholdSeconds int64      `json:"threshold_seconds,omitempty"`
	Reason           string     `json:"reason,omitempty"`
}

// Attribution credits a provider (BR-ATTR-01; configured, never hardcoded).
type Attribution struct {
	Provider string `json:"provider"`
	Text     string `json:"text"`
	URL      string `json:"url"`
}

// Warning is a partial-result warning entry (closed code enum).
type Warning struct {
	ProviderID string `json:"provider_id,omitempty"`
	Code       string `json:"code"` // provider_unavailable | stale
	Message    string `json:"message"`
	Since      string `json:"since,omitempty"`
}

// Pagination is keyset pagination metadata (no total_count; API-02).
type Pagination struct {
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
}

// Options customize an envelope response.
type Options struct {
	RequestID   string
	Timezone    string
	Units       map[string]string
	Freshness   *Freshness
	Attribution []Attribution
	Warnings    []Warning
	Pagination  *Pagination
}

// JSON writes data wrapped in the envelope with status code.
func JSON(c *gin.Context, status int, data any, opts Options) {
	env := Envelope{
		Data: data,
		Metadata: &Metadata{
			RequestID:   opts.RequestID,
			GeneratedAt: time.Now().UTC(),
			Timezone:    opts.Timezone,
			Units:       opts.Units,
		},
		Freshness:   opts.Freshness,
		Attribution: opts.Attribution,
		Warnings:    opts.Warnings,
		Pagination:  opts.Pagination,
	}
	if len(opts.Warnings) > 0 {
		env.PartialResult = true
	}
	c.JSON(status, env)
}

// OK writes a 200 envelope.
func OK(c *gin.Context, data any, opts Options) { JSON(c, http.StatusOK, data, opts) }

// Created writes a 201 envelope.
func Created(c *gin.Context, data any, opts Options) { JSON(c, http.StatusCreated, data, opts) }
