package promexport

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/forecastiq/forecastiq/internal/admin"
)

type fakeReader struct {
	backup  *admin.BackupStatus
	restore *admin.BackupStatus
	err     error
}

func (f *fakeReader) Read() (*admin.BackupStatus, *admin.BackupStatus, error) {
	return f.backup, f.restore, f.err
}

func TestBackupCollector_AbsentFileExportsNothing(t *testing.T) {
	c := NewBackupCollector(&fakeReader{})
	reg := prometheus.NewRegistry()
	require.NoError(t, reg.Register(c))

	families, err := reg.Gather()
	require.NoError(t, err)
	assert.Empty(t, families, "no metrics must be exported before the status file exists (A10/A11 stay silent pre-WP-24)")
}

func TestBackupCollector_ReadErrorExportsNothing(t *testing.T) {
	c := NewBackupCollector(&fakeReader{err: errors.New("boom")})
	reg := prometheus.NewRegistry()
	require.NoError(t, reg.Register(c))

	families, err := reg.Gather()
	require.NoError(t, err)
	assert.Empty(t, families)
}

func TestBackupCollector_ExportsSuccessTimestampAndStatus(t *testing.T) {
	completed := time.Date(2026, 7, 26, 3, 0, 0, 0, time.UTC)
	restored := time.Date(2026, 7, 1, 4, 0, 0, 0, time.UTC)
	c := NewBackupCollector(&fakeReader{
		backup:  &admin.BackupStatus{CompletedAt: &completed, Status: "success"},
		restore: &admin.BackupStatus{CompletedAt: &restored, Status: "success"},
	})

	expected := strings.NewReader(`
# HELP forecastiq_backup_last_success_timestamp_seconds Unix timestamp of the last successful backup (from the backup status file).
# TYPE forecastiq_backup_last_success_timestamp_seconds gauge
forecastiq_backup_last_success_timestamp_seconds 1.7850348e+09
# HELP forecastiq_backup_status Last backup status (1 = success, 0 = failure).
# TYPE forecastiq_backup_status gauge
forecastiq_backup_status 1
# HELP forecastiq_restore_test_last_timestamp_seconds Unix timestamp of the last restore test (from the backup status file).
# TYPE forecastiq_restore_test_last_timestamp_seconds gauge
forecastiq_restore_test_last_timestamp_seconds 1.7828784e+09
# HELP forecastiq_restore_test_status Last restore-test status (1 = success, 0 = failure).
# TYPE forecastiq_restore_test_status gauge
forecastiq_restore_test_status 1
`)
	require.NoError(t, testutil.CollectAndCompare(c, expected))
}

func TestBackupCollector_FailedRestoreTestExportsStatusZero(t *testing.T) {
	completed := time.Date(2026, 7, 26, 3, 0, 0, 0, time.UTC)
	restored := time.Date(2026, 8, 1, 4, 0, 0, 0, time.UTC)
	c := NewBackupCollector(&fakeReader{
		backup:  &admin.BackupStatus{CompletedAt: &completed, Status: "success"},
		restore: &admin.BackupStatus{CompletedAt: &restored, Status: "failed"},
	})
	// A failed restore test still refreshes the timestamp, so A11b must key on
	// forecastiq_restore_test_status == 0 (DRB-WP24-004).
	expected := strings.NewReader(`
# HELP forecastiq_restore_test_status Last restore-test status (1 = success, 0 = failure).
# TYPE forecastiq_restore_test_status gauge
forecastiq_restore_test_status 0
`)
	require.NoError(t, testutil.CollectAndCompare(c, expected, "forecastiq_restore_test_status"))
}

func TestBackupCollector_FailedBackupExportsStatusZeroWithoutTimestamp(t *testing.T) {
	completed := time.Date(2026, 7, 26, 3, 0, 0, 0, time.UTC)
	c := NewBackupCollector(&fakeReader{
		backup: &admin.BackupStatus{CompletedAt: &completed, Status: "failed"},
	})

	expected := strings.NewReader(`
# HELP forecastiq_backup_status Last backup status (1 = success, 0 = failure).
# TYPE forecastiq_backup_status gauge
forecastiq_backup_status 0
`)
	require.NoError(t, testutil.CollectAndCompare(c, expected))
}

func TestEngineCollector_DescribesCatalogMetrics(t *testing.T) {
	c := NewEngineCollector(nil)
	ch := make(chan *prometheus.Desc, 8)
	c.Describe(ch)
	close(ch)

	var names []string
	for d := range ch {
		names = append(names, d.String())
	}
	joined := strings.Join(names, "\n")
	for _, want := range []string{"engine_lag_seconds", "evaluation_backlog", "ranking_freshness_age_seconds"} {
		assert.Contains(t, joined, want)
	}
}
