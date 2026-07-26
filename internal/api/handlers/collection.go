package handlers

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/forecastiq/forecastiq/internal/api/respond"
	"github.com/forecastiq/forecastiq/internal/collection"
	collectiondomain "github.com/forecastiq/forecastiq/internal/collection/domain"
)

// TriggerCollection godoc
// @Summary      Trigger an immediate collection (admin)
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        body body TriggerCollectionRequest true "trigger"
// @Success      200 {object} respond.Envelope
// @Failure      401 {object} respond.Problem
// @Failure      409 {object} respond.Problem
// @Failure      422 {object} respond.Problem
// @Router       /admin/collections/trigger [post]
func (h *Handlers) TriggerCollection(c *gin.Context) {
	var req TriggerCollectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Error(c, &badField{detail: err.Error()}, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	providerID, err := parseUUIDParam(req.ProviderID, "provider_id")
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	locationID, err := parseUUIDParam(req.LocationID, "location_id")
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	ctx := c.Request.Context()
	provider, err := h.Providers.GetProvider(ctx, providerID)
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	location, err := h.Locations.GetLocation(ctx, locationID)
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	config, err := h.Configs.GetConfigurationByProviderID(ctx, providerID)
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	coll, err := h.Collector.Collect(ctx, collection.CollectInput{
		Provider: provider, Location: location, Config: config,
		Actor: actor(c), Source: collection.SourceManual,
	})
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	// Budget/rate guard (workflow 05 §6): a rate-limited outcome is reported as
	// 429 with Retry-After rather than a 200 success envelope. The circuit-open
	// guard is a 409 surfaced as a CircuitOpenError above.
	if coll.Status == collectiondomain.StatusRateLimited {
		c.Header("Retry-After", "60")
		respond.Error(c, respond.ErrRateLimited, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	respond.OK(c, collectionDTO(coll), respond.Options{RequestID: respond.RequestID(c)})
}

// ReplayCollection godoc
// @Summary      Replay a stored collection payload through the current adapter (admin)
// @Tags         admin
// @Produce      json
// @Param        id path string true "collection id"
// @Success      200 {object} respond.Envelope
// @Failure      401 {object} respond.Problem
// @Failure      404 {object} respond.Problem
// @Failure      422 {object} respond.Problem
// @Router       /admin/collections/{id}/replay [post]
func (h *Handlers) ReplayCollection(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	coll, err := h.Replayer.Replay(c.Request.Context(), id, actor(c))
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	respond.OK(c, collectionDTO(coll), respond.Options{RequestID: respond.RequestID(c)})
}

// ListCollections godoc
// @Summary      List forecast collections (admin lineage/health query)
// @Tags         admin
// @Produce      json
// @Param        provider_id query string false "provider id"
// @Param        location_id query string false "location id"
// @Param        status query string false "collection status"
// @Param        cursor query string false "pagination cursor"
// @Param        limit query int false "page size" default(50)
// @Success      200 {object} respond.Envelope
// @Failure      401 {object} respond.Problem
// @Router       /forecast-collections [get]
func (h *Handlers) ListCollections(c *gin.Context) {
	in := collection.CollectionListInput{Limit: 50}
	if id, ok := queryUUID(c, "provider_id"); !ok {
		return
	} else {
		in.ProviderID = id
	}
	if id, ok := queryUUID(c, "location_id"); !ok {
		return
	} else {
		in.LocationID = id
	}
	if s := c.Query("status"); s != "" {
		status := collectiondomain.CollectionStatus(s)
		in.Status = &status
	}
	in.Cursor = c.Query("cursor")
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			in.Limit = n
		}
	}
	collections, page, err := h.Reader.ListCollections(c.Request.Context(), in)
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	dtos := make([]CollectionDTO, 0, len(collections))
	for _, coll := range collections {
		dtos = append(dtos, collectionDTO(coll))
	}
	respond.OK(c, gin.H{"collections": dtos}, respond.Options{
		RequestID:  respond.RequestID(c),
		Pagination: &respond.Pagination{NextCursor: page.NextCursor, HasMore: page.HasMore},
	})
}

// GetCollection godoc
// @Summary      Get a forecast collection (admin)
// @Tags         admin
// @Produce      json
// @Param        id path string true "collection id"
// @Success      200 {object} respond.Envelope
// @Failure      404 {object} respond.Problem
// @Router       /forecast-collections/{id} [get]
func (h *Handlers) GetCollection(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	coll, err := h.Reader.GetCollection(c.Request.Context(), id)
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	respond.OK(c, collectionDTO(coll), respond.Options{RequestID: respond.RequestID(c)})
}

// CollectionSnapshots godoc
// @Summary      Snapshots of a forecast collection (admin)
// @Description  Returns the collection's lineage row plus all its stored
// @Description  snapshots — the historical raw-data view behind /forecasts/latest.
// @Tags         admin
// @Produce      json
// @Param        id path string true "collection id"
// @Success      200 {object} respond.Envelope
// @Failure      404 {object} respond.Problem
// @Router       /forecast-collections/{id}/snapshots [get]
func (h *Handlers) CollectionSnapshots(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	ctx := c.Request.Context()
	coll, err := h.Reader.GetCollection(ctx, id)
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	snapshots, err := h.Reader.SnapshotsByCollection(ctx, id)
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	snaps := make([]SnapshotDTO, 0, len(snapshots))
	for _, s := range snapshots {
		snaps = append(snaps, snapshotDTO(s))
	}
	respond.OK(c, gin.H{"collection": collectionDTO(coll), "snapshots": snaps},
		respond.Options{RequestID: respond.RequestID(c)})
}

// LatestForecast godoc
// @Summary      Latest forecast for a provider + location
// @Tags         data
// @Produce      json
// @Param        location_id query string true "location id"
// @Param        provider_id query string true "provider id"
// @Success      200 {object} respond.Envelope
// @Failure      404 {object} respond.Problem
// @Failure      422 {object} respond.Problem
// @Router       /forecasts/latest [get]
func (h *Handlers) LatestForecast(c *gin.Context) {
	locationID, ok := queryUUID(c, "location_id")
	if !ok {
		return
	}
	providerID, ok := queryUUID(c, "provider_id")
	if !ok {
		return
	}
	if locationID == nil || providerID == nil {
		respond.Error(c, &badField{detail: "location_id and provider_id are required"}, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	ctx := c.Request.Context()
	latest, err := h.Reader.LatestForecast(ctx, *providerID, *locationID)
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	if latest == nil {
		respond.Error(c, collectiondomain.ErrNotFound, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	provider, err := h.Providers.GetProvider(ctx, *providerID)
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	snaps := make([]SnapshotDTO, 0, len(latest.Snapshots))
	for _, s := range latest.Snapshots {
		snaps = append(snaps, snapshotDTO(s))
	}
	opts := respond.Options{
		RequestID: respond.RequestID(c),
		Attribution: []respond.Attribution{{
			Provider: provider.Name, Text: provider.AttributionText, URL: provider.AttributionURL,
		}},
		Freshness: collectionFreshness(latest.Collection),
		Units: map[string]string{
			"temperature": "°C", "precipitation": "mm", "wind_speed": "m/s",
			"pressure": "hPa", "humidity": "%",
		},
	}
	respond.OK(c, gin.H{"collection": collectionDTO(latest.Collection), "snapshots": snaps}, opts)
}

// collectionFreshness computes the BR-FRESH forecast-collection freshness
// state from the collection's completion time (75 / 180 min / 24 h).
func collectionFreshness(coll *collectiondomain.ForecastCollection) *respond.Freshness {
	if coll.CompletedAt == nil {
		return &respond.Freshness{State: "unavailable", Reason: "no_completed_collection"}
	}
	age := time.Since(*coll.CompletedAt)
	state := "fresh"
	switch {
	case age > 24*time.Hour:
		state = "stale"
	case age > 180*time.Minute:
		state = "delayed"
	}
	return &respond.Freshness{
		State: state, LastUpdated: coll.CompletedAt,
		AgeSeconds: int64(age.Seconds()), ThresholdSeconds: 75 * 60,
	}
}
