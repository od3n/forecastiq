package handlers

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/admin"
	"github.com/forecastiq/forecastiq/internal/api/respond"
)

// Collection-cell freshness thresholds (S-10; BR-FRESH: a cell is stale past
// ~180 m without a successful collection).
const (
	cellFreshMax = 90 * time.Minute
	cellStaleMax = 180 * time.Minute
)

// ── DTOs ────────────────────────────────────────────────────────────────

type healthCellDTO struct {
	Provider        providerRef        `json:"provider"`
	LocationID      string             `json:"location_id"`
	LocationName    string             `json:"location_name"`
	LastSuccessAt   *time.Time         `json:"last_success_at,omitempty"`
	LastStatus      string             `json:"last_status,omitempty"`
	NextScheduledAt *time.Time         `json:"next_scheduled_at,omitempty"`
	Freshness       *respond.Freshness `json:"freshness"`
}

type healthCircuitDTO struct {
	ProviderID          string     `json:"provider_id"`
	ProviderName        string     `json:"provider_name"`
	State               string     `json:"state"`
	ConsecutiveFailures int        `json:"consecutive_failures"`
	NextProbeAt         *time.Time `json:"next_probe_at,omitempty"`
}

type healthObsLocationDTO struct {
	LocationID      string     `json:"location_id"`
	LocationName    string     `json:"location_name"`
	LastObservedAt  *time.Time `json:"last_observed_at,omitempty"`
	SuspectCount24h int        `json:"suspect_count_24h"`
}

type observationCollectorDTO struct {
	Locations        []healthObsLocationDTO `json:"locations"`
	LocationsCovered int                    `json:"locations_covered"`
}

type volumeDTO struct {
	UsedBytes  uint64  `json:"used_bytes"`
	TotalBytes uint64  `json:"total_bytes"`
	UsedPct    float64 `json:"used_pct"`
}

type backupStatusDTO struct {
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Status      string     `json:"status"`
}

type systemHealthDTO struct {
	PayloadVolume    *volumeDTO       `json:"payload_volume,omitempty"`
	EngineLagSeconds *int64           `json:"engine_lag_seconds,omitempty"`
	LastBackup       *backupStatusDTO `json:"last_backup,omitempty"`
	LastRestoreTest  *backupStatusDTO `json:"last_restore_test,omitempty"`
}

type adminHealthDTO struct {
	Cells                []healthCellDTO         `json:"cells"`
	Circuits             []healthCircuitDTO      `json:"circuits"`
	ObservationCollector observationCollectorDTO `json:"observation_collector"`
	System               systemHealthDTO         `json:"system"`
}

// AdminHealth godoc
// @Summary      Collection health (S-10, admin)
// @Description  Operational triage view: per provider×location cell (last
// @Description  success, next scheduled, freshness), provider circuits, the
// @Description  observation collector, and the system section (payload volume,
// @Description  engine lag, backup/restore status). Served purely from
// @Description  application tables + statfs + the backup status file. Admin only.
// @Tags         admin
// @Produce      json
// @Param        provider_id query string false "filter cells/circuits by provider"
// @Param        location_id query string false "filter cells by location"
// @Param        status      query string false "filter cells by freshness state (fresh|delayed|stale|unavailable)"
// @Success      200 {object} respond.Envelope
// @Failure      401 {object} respond.Problem
// @Failure      403 {object} respond.Problem
// @Router       /admin/health [get]
func (h *Handlers) AdminHealth(c *gin.Context) {
	providerID, ok := queryUUID(c, "provider_id")
	if !ok {
		return
	}
	locationID, ok := queryUUID(c, "location_id")
	if !ok {
		return
	}
	statusFilter := c.Query("status")

	hv, err := h.AdminHealthReader.Assemble(c.Request.Context())
	if err != nil {
		respond.Error(c, err, respond.RequestID(c), c.Request.URL.Path)
		return
	}
	now := time.Now().UTC()

	// Providers with an open circuit render their cells as unavailable.
	openCircuit := map[uuid.UUID]bool{}
	for _, cir := range hv.Circuits {
		if cir.State == "open" {
			openCircuit[cir.ProviderID] = true
		}
	}

	cells := make([]healthCellDTO, 0, len(hv.Cells))
	for _, cell := range hv.Cells {
		if providerID != nil && cell.ProviderID != *providerID {
			continue
		}
		if locationID != nil && cell.LocationID != *locationID {
			continue
		}
		reason := ""
		if openCircuit[cell.ProviderID] {
			reason = "circuit_open"
		}
		var fresh *respond.Freshness
		if reason != "" && cell.LastSuccessAt == nil {
			fresh = &respond.Freshness{State: respond.FreshnessUnavailable, Reason: reason}
		} else {
			fresh = respond.ComputeFreshness(cell.LastSuccessAt, now, cellFreshMax, cellStaleMax, "never_collected")
			if reason != "" {
				fresh.State = respond.FreshnessUnavailable
				fresh.Reason = reason
			}
		}
		if statusFilter != "" && fresh.State != statusFilter {
			continue
		}
		cells = append(cells, healthCellDTO{
			Provider:        providerRef{ID: cell.ProviderID.String(), Name: cell.ProviderName, Slug: cell.ProviderSlug},
			LocationID:      cell.LocationID.String(),
			LocationName:    cell.LocationName,
			LastSuccessAt:   utcPtr(cell.LastSuccessAt),
			LastStatus:      cell.LastStatus,
			NextScheduledAt: utcPtr(cell.NextScheduledAt),
			Freshness:       fresh,
		})
	}

	circuits := make([]healthCircuitDTO, 0, len(hv.Circuits))
	for _, cir := range hv.Circuits {
		if providerID != nil && cir.ProviderID != *providerID {
			continue
		}
		circuits = append(circuits, healthCircuitDTO{
			ProviderID: cir.ProviderID.String(), ProviderName: cir.ProviderName,
			State: cir.State, ConsecutiveFailures: cir.ConsecutiveFailures, NextProbeAt: utcPtr(cir.NextProbeAt),
		})
	}

	obsLocations := make([]healthObsLocationDTO, 0, len(hv.ObservationLocations))
	covered := 0
	for _, o := range hv.ObservationLocations {
		if locationID != nil && o.LocationID != *locationID {
			continue
		}
		if o.LastObservedAt != nil {
			covered++
		}
		obsLocations = append(obsLocations, healthObsLocationDTO{
			LocationID: o.LocationID.String(), LocationName: o.LocationName,
			LastObservedAt: utcPtr(o.LastObservedAt), SuspectCount24h: o.SuspectCount24h,
		})
	}

	dto := adminHealthDTO{
		Cells:                cells,
		Circuits:             circuits,
		ObservationCollector: observationCollectorDTO{Locations: obsLocations, LocationsCovered: covered},
		System: systemHealthDTO{
			EngineLagSeconds: hv.EngineLagSeconds,
			PayloadVolume:    volumeToDTO(hv.Volume),
			LastBackup:       backupToDTO(hv.LastBackup),
			LastRestoreTest:  backupToDTO(hv.LastRestoreTest),
		},
	}

	// Admin operational data is never cached (conventions §6): 60 s polling
	// hits cheap assembly directly.
	c.Header("Cache-Control", "no-store")
	respond.OK(c, dto, respond.Options{RequestID: respond.RequestID(c)})
}

func volumeToDTO(v *admin.VolumeUsage) *volumeDTO {
	if v == nil {
		return nil
	}
	pct := v.UsedPct
	if r := respond.RoundScore(&pct); r != nil {
		pct = *r
	}
	return &volumeDTO{UsedBytes: v.UsedBytes, TotalBytes: v.TotalBytes, UsedPct: pct}
}

func backupToDTO(b *admin.BackupStatus) *backupStatusDTO {
	if b == nil {
		return nil
	}
	return &backupStatusDTO{CompletedAt: utcPtr(b.CompletedAt), Status: b.Status}
}
