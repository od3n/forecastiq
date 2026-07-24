package handlers

import (
	"github.com/gin-gonic/gin"

	"github.com/forecastiq/forecastiq/internal/admin"
	"github.com/forecastiq/forecastiq/internal/api/respond"
)

// AdminRecompute godoc
// @Summary      Recompute analysis (S-13, admin)
// @Description  Runs the analysis pipeline on demand (match → aggregate →
// @Description  rank), e.g. after a correction or methodology change. Audited.
// @Tags         admin
// @Produce      json
// @Success      200 {object} respond.Envelope
// @Failure      401 {object} respond.Problem
// @Failure      403 {object} respond.Problem
// @Router       /admin/recompute [post]
func (h *Handlers) AdminRecompute(c *gin.Context) {
	p, _ := respond.PrincipalFrom(c)
	affected, err := h.Recompute.Recompute(c.Request.Context(), admin.RecomputeActor{
		UserID: p.UserID, Name: p.Name, IPAddress: c.ClientIP(),
	})
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	c.Header("Cache-Control", "no-store")
	respond.OK(c, gin.H{"records_affected": affected}, respond.Options{RequestID: respond.RequestID(c)})
}
