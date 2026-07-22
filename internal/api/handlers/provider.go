package handlers

import (
	"github.com/gin-gonic/gin"

	"github.com/forecastiq/forecastiq/internal/api/respond"
)

// ListProviders godoc
// @Summary      List providers
// @Description  Returns providers with attribution (BR-ATTR-01).
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
	dtos := make([]ProviderDTO, 0, len(providers))
	for _, p := range providers {
		dtos = append(dtos, providerDTO(p))
	}
	respond.OK(c, gin.H{"providers": dtos}, respond.Options{RequestID: respond.RequestID(c)})
}
