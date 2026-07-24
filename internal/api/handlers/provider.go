package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/api/respond"
	"github.com/forecastiq/forecastiq/internal/catalog"
	"github.com/forecastiq/forecastiq/internal/collection"
)

// ListProviders godoc
// @Summary      List providers
// @Description  Returns providers with attribution (BR-ATTR-01), plus the
// @Description  non-sensitive lineage fields adapter_version (latest successful
// @Description  collection) and collecting_since (S-03/S-11; §4.1).
// @Tags         catalog
// @Produce      json
// @Success      200 {object} respond.Envelope
// @Router       /providers [get]
func (h *Handlers) ListProviders(c *gin.Context) {
	providers, err := h.Providers.ListProviders(c.Request.Context())
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	lineages, err := h.Reader.ProviderLineages(c.Request.Context())
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	dtos := make([]ProviderDTO, 0, len(providers))
	var attribution []respond.Attribution
	for _, p := range providers {
		dtos = append(dtos, enrichedProviderDTO(p, lineages))
		attribution = append(attribution, respond.Attribution{Provider: p.Name, Text: p.AttributionText, URL: p.AttributionURL})
	}
	respond.OK(c, gin.H{"providers": dtos}, respond.Options{RequestID: respond.RequestID(c), Attribution: attribution})
}

// GetProvider godoc
// @Summary      Get a provider
// @Description  Returns one provider with attribution + lineage (adapter_version,
// @Description  collecting_since) for the S-03 detail header (§4.1).
// @Tags         catalog
// @Produce      json
// @Param        id path string true "provider id"
// @Success      200 {object} respond.Envelope
// @Failure      404 {object} respond.Problem
// @Router       /providers/{id} [get]
func (h *Handlers) GetProvider(c *gin.Context) {
	id, ok := pathUUID(c, "id")
	if !ok {
		return
	}
	provider, err := h.Providers.GetProvider(c.Request.Context(), id)
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	lineages, err := h.Reader.ProviderLineages(c.Request.Context())
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	respond.OK(c, enrichedProviderDTO(provider, lineages), respond.Options{
		RequestID:   respond.RequestID(c),
		Attribution: []respond.Attribution{{Provider: provider.Name, Text: provider.AttributionText, URL: provider.AttributionURL}},
	})
}

// enrichedProviderDTO builds the public provider DTO with lineage folded in.
func enrichedProviderDTO(p *catalog.Provider, lineages map[uuid.UUID]collection.ProviderLineage) ProviderDTO {
	dto := providerDTO(p)
	if l, ok := lineages[p.ID]; ok {
		dto.AdapterVersion = l.AdapterVersion
		dto.CollectingSince = l.CollectingSince
	}
	return dto
}
