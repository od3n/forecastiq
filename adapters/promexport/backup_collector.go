package promexport

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/forecastiq/forecastiq/internal/admin"
)

var (
	backupLastSuccessDesc = prometheus.NewDesc(
		"forecastiq_backup_last_success_timestamp_seconds",
		"Unix timestamp of the last successful backup (from the backup status file).",
		nil, nil)
	backupStatusDesc = prometheus.NewDesc(
		"forecastiq_backup_status",
		"Last backup status (1 = success, 0 = failure).",
		nil, nil)
	restoreTestLastDesc = prometheus.NewDesc(
		"forecastiq_restore_test_last_timestamp_seconds",
		"Unix timestamp of the last restore test (from the backup status file).",
		nil, nil)
)

// BackupStatusReader yields the last backup / restore-test entries. Implemented
// by adapters/backupstatus.FileReader (nil entries when the file is absent).
type BackupStatusReader interface {
	Read() (lastBackup, lastRestoreTest *admin.BackupStatus, err error)
}

// BackupCollector exports A10/A11 backup metrics from the status file written
// by the WP-24 backup/restore scripts. Until that file exists the metrics are
// absent, so BackupFailure / RestoreTestOverdue stay silent pre-WP-24 and
// become live the moment the first backup completes — no fabricated zeros
// that would page immediately on a fresh deployment (DRB-WP22-006).
type BackupCollector struct {
	reader BackupStatusReader
}

// NewBackupCollector returns a collector over the given status reader.
func NewBackupCollector(reader BackupStatusReader) *BackupCollector {
	return &BackupCollector{reader: reader}
}

// Describe implements prometheus.Collector.
func (c *BackupCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- backupLastSuccessDesc
	ch <- backupStatusDesc
	ch <- restoreTestLastDesc
}

// Collect implements prometheus.Collector.
func (c *BackupCollector) Collect(ch chan<- prometheus.Metric) {
	lastBackup, lastRestore, err := c.reader.Read()
	if err != nil {
		return
	}
	if lastBackup != nil {
		status := 0.0
		if lastBackup.Status == "success" {
			status = 1.0
		}
		ch <- prometheus.MustNewConstMetric(backupStatusDesc, prometheus.GaugeValue, status)
		if lastBackup.CompletedAt != nil && status == 1.0 {
			ch <- prometheus.MustNewConstMetric(backupLastSuccessDesc, prometheus.GaugeValue,
				float64(lastBackup.CompletedAt.Unix()))
		}
	}
	if lastRestore != nil && lastRestore.CompletedAt != nil {
		ch <- prometheus.MustNewConstMetric(restoreTestLastDesc, prometheus.GaugeValue,
			float64(lastRestore.CompletedAt.Unix()))
	}
}
