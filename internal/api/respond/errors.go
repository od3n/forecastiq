package respond

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	catalogdomain "github.com/forecastiq/forecastiq/internal/catalog/domain"
	collectiondomain "github.com/forecastiq/forecastiq/internal/collection/domain"
)

// Error type base URL (docs anchor per error class).
const errorBase = "https://forecastiq.example/errors/"

// API-layer sentinel errors mapped to their taxonomy class.
var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrRateLimited  = errors.New("rate_limited")
)

// Problem is the RFC 7807 error envelope with ForecastIQ extensions.
type Problem struct {
	Type             string            `json:"type"`
	Title            string            `json:"title"`
	Status           int               `json:"status"`
	Detail           string            `json:"detail"`
	Instance         string            `json:"instance,omitempty"`
	RequestID        string            `json:"request_id"`
	Retryable        bool              `json:"retryable"`
	Docs             string            `json:"docs,omitempty"`
	Errors           []FieldError      `json:"errors,omitempty"`
	ExistingResource *ExistingResource `json:"existing_resource,omitempty"`
}

// FieldError is a field-level validation detail.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ExistingResource references the conflicting resource on a duplicate error.
type ExistingResource struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	DistanceDegrees float64 `json:"distance_degrees"`
}

// fieldMessage is implemented by single-field validation errors (e.g. the
// collection reader's query validation).
type fieldMessage interface {
	Field() string
	Message() string
}

// Error writes err as an RFC 7807 problem (application/problem+json), mapping
// domain errors to their taxonomy class. Provider internals are never echoed.
func Error(c *gin.Context, err error, requestID, instance string) {
	status, p := Classify(err, requestID, instance)
	body, _ := json.Marshal(p)
	c.Header("Content-Type", "application/problem+json; charset=utf-8")
	c.AbortWithStatusJSON(status, json.RawMessage(body))
}

// Classify maps an error to its HTTP status + Problem representation.
func Classify(err error, requestID, instance string) (int, Problem) {
	p := Problem{RequestID: requestID, Instance: instance}

	var valErr *catalogdomain.ValidationError
	if errors.As(err, &valErr) {
		p.Type = errorBase + "validation"
		p.Title = "Validation Error"
		p.Status = http.StatusUnprocessableEntity
		p.Detail = valErr.Error()
		p.Retryable = true
		for _, f := range valErr.Fields {
			p.Errors = append(p.Errors, FieldError{Field: f.Field, Message: f.Message})
		}
		return p.Status, p
	}

	var fm fieldMessage
	if errors.As(err, &fm) {
		p.Type = errorBase + "validation"
		p.Title = "Validation Error"
		p.Status = http.StatusUnprocessableEntity
		p.Detail = fm.Message()
		p.Retryable = true
		p.Errors = []FieldError{{Field: fm.Field(), Message: fm.Message()}}
		return p.Status, p
	}

	var dupErr *catalogdomain.DuplicateLocationError
	if errors.As(err, &dupErr) {
		p.Type = errorBase + "duplicate"
		p.Title = "Duplicate Location"
		p.Status = http.StatusConflict
		p.Detail = dupErr.Error()
		p.Retryable = true
		p.ExistingResource = &ExistingResource{
			ID: dupErr.ExistingID.String(), Name: dupErr.ExistingName, DistanceDegrees: dupErr.DistanceDegrees,
		}
		return p.Status, p
	}

	var nameErr *catalogdomain.NameConflictError
	if errors.As(err, &nameErr) {
		p.Type = errorBase + "conflict"
		p.Title = "Name Conflict"
		p.Status = http.StatusConflict
		p.Detail = nameErr.Error()
		p.Retryable = true
		return p.Status, p
	}

	var circuitErr *collectiondomain.CircuitOpenError
	if errors.As(err, &circuitErr) {
		p.Type = errorBase + "conflict"
		p.Title = "Provider Circuit Open"
		p.Status = http.StatusConflict
		p.Detail = "The provider circuit breaker is open; collection is temporarily blocked."
		p.Retryable = true
		return p.Status, p
	}

	if errors.Is(err, catalogdomain.ErrNotFound) || errors.Is(err, collectiondomain.ErrNotFound) {
		p.Type = errorBase + "not_found"
		p.Title = "Not Found"
		p.Status = http.StatusNotFound
		p.Detail = "The requested resource was not found."
		return p.Status, p
	}

	if errors.Is(err, catalogdomain.ErrInactive) || errors.Is(err, collectiondomain.ErrInactive) {
		p.Type = errorBase + "validation"
		p.Title = "Resource Inactive"
		p.Status = http.StatusUnprocessableEntity
		p.Detail = "The provider, location, or configuration is not active."
		return p.Status, p
	}

	if errors.Is(err, collectiondomain.ErrPayloadUnavailable) {
		p.Type = errorBase + "payload_unavailable"
		p.Title = "Payload Unavailable"
		p.Status = http.StatusUnprocessableEntity
		p.Detail = "The raw payload is unavailable (expired or corrupt)."
		return p.Status, p
	}

	if errors.Is(err, ErrUnauthorized) {
		p.Type = errorBase + "unauthorized"
		p.Title = "Unauthorized"
		p.Status = http.StatusUnauthorized
		p.Detail = "Authentication is required to access this resource."
		p.Retryable = true
		return p.Status, p
	}

	if errors.Is(err, ErrForbidden) {
		p.Type = errorBase + "forbidden"
		p.Title = "Forbidden"
		p.Status = http.StatusForbidden
		p.Detail = "You do not have permission to perform this action."
		return p.Status, p
	}

	if errors.Is(err, ErrRateLimited) {
		p.Type = errorBase + "rate_limited"
		p.Title = "Rate Limited"
		p.Status = http.StatusTooManyRequests
		p.Detail = "Too many requests; please retry later."
		p.Retryable = true
		return p.Status, p
	}

	// Default: sanitized internal error (no stack/SQL/provider internals).
	p.Type = errorBase + "internal"
	p.Title = "Internal Server Error"
	p.Status = http.StatusInternalServerError
	p.Detail = "An unexpected error occurred."
	p.Retryable = true
	return p.Status, p
}

// NotFound writes a 404 problem (unknown route/resource).
func NotFound(c *gin.Context, requestID, instance string) {
	Error(c, catalogdomain.ErrNotFound, requestID, instance)
}
