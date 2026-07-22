package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Healthz godoc
// @Summary      Liveness probe
// @Tags         operations
// @Produce      json
// @Success      200 {object} map[string]string
// @Router       /healthz [get]
func (h *Handlers) Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Readyz godoc
// @Summary      Readiness probe (DB + payload volume)
// @Tags         operations
// @Produce      json
// @Success      200 {object} map[string]any
// @Failure      503 {object} map[string]any
// @Router       /readyz [get]
func (h *Handlers) Readyz(c *gin.Context) {
	results, allOK := h.Health.RunAll(c.Request.Context())
	checks := make([]gin.H, 0, len(results))
	for _, r := range results {
		checks = append(checks, gin.H{"name": r.Name, "healthy": r.Healthy, "error": r.Error})
	}
	status := http.StatusOK
	state := "ready"
	if !allOK {
		status = http.StatusServiceUnavailable
		state = "not_ready"
	}
	c.JSON(status, gin.H{"status": state, "checks": checks})
}
