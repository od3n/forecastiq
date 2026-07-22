package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/forecastiq/forecastiq/internal/api/respond"
	"github.com/forecastiq/forecastiq/internal/catalog"
)

// ListLocations godoc
// @Summary      List locations
// @Description  Returns locations (keyset-paginated; filter by active status).
// @Tags         catalog
// @Produce      json
// @Param        active query bool false "filter by active status"
// @Param        cursor query string false "pagination cursor (last id)"
// @Param        limit  query int false "page size (1..200)" default(50)
// @Success      200 {object} respond.Envelope
// @Failure      422 {object} respond.Problem
// @Router       /locations [get]
func (h *Handlers) ListLocations(c *gin.Context) {
	in := catalog.ListLocationsInput{Limit: 50}
	if v := c.Query("active"); v != "" {
		b, _ := strconv.ParseBool(v)
		in.Active = &b
	}
	in.Cursor = c.Query("cursor")
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			in.Limit = n
		}
	}
	locations, page, err := h.Locations.ListLocations(c.Request.Context(), in)
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	dtos := make([]LocationDTO, 0, len(locations))
	for _, l := range locations {
		dtos = append(dtos, locationDTO(l))
	}
	respond.OK(c, gin.H{"locations": dtos}, respond.Options{
		RequestID:  respond.RequestID(c),
		Pagination: &respond.Pagination{NextCursor: page.NextCursor, HasMore: page.HasMore},
	})
}

// GetLocation godoc
// @Summary      Get a location
// @Tags         catalog
// @Produce      json
// @Param        id path string true "location id"
// @Success      200 {object} respond.Envelope
// @Failure      404 {object} respond.Problem
// @Router       /locations/{id} [get]
func (h *Handlers) GetLocation(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	loc, err := h.Locations.GetLocation(c.Request.Context(), id)
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	respond.OK(c, locationDTO(loc), respond.Options{RequestID: respond.RequestID(c), Timezone: loc.Timezone})
}

// CreateLocation godoc
// @Summary      Create a location (admin)
// @Tags         catalog
// @Accept       json
// @Produce      json
// @Param        body body CreateLocationRequest true "location"
// @Success      201 {object} respond.Envelope
// @Failure      401 {object} respond.Problem
// @Failure      409 {object} respond.Problem
// @Failure      422 {object} respond.Problem
// @Router       /locations [post]
func (h *Handlers) CreateLocation(c *gin.Context) {
	var req CreateLocationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Error(c, &badField{detail: err.Error()}, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	loc, err := h.Locations.CreateLocation(c.Request.Context(), catalog.CreateLocationInput{
		Name:               req.Name,
		Latitude:           req.Latitude,
		Longitude:          req.Longitude,
		CountryCode:        req.CountryCode,
		Timezone:           req.Timezone,
		AllowNearDuplicate: req.AllowNearDuplicate,
		Actor:              actor(c),
	})
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	respond.Created(c, locationDTO(loc), respond.Options{RequestID: respond.RequestID(c)})
}

// badField maps a binding error to a validation problem.
type badField struct{ detail string }

func (e *badField) Error() string   { return e.detail }
func (e *badField) Field() string   { return "body" }
func (e *badField) Message() string { return e.detail }
