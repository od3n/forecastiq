//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/forecastiq/forecastiq/adapters/persistence/analysispg"
	"github.com/forecastiq/forecastiq/internal/analysis"
	analysisdomain "github.com/forecastiq/forecastiq/internal/analysis/domain"
	catalogdomain "github.com/forecastiq/forecastiq/internal/catalog/domain"
	"github.com/forecastiq/forecastiq/internal/platform/clock"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
	"github.com/forecastiq/forecastiq/internal/platform/ids"
	"github.com/forecastiq/forecastiq/internal/platform/metrics"
)

const aggHorizon = 1440

// insertTempPair seeds a full temperature pair: a forecast_collection + snapshot
// (temperature_c = fTemp) and an observation (temperature_c = oTemp), joined by a
// matched_evaluations row.
func insertTempPair(ctx context.Context, t *testing.T, pool *pgxpool.Pool, target time.Time, fTemp, oTemp float64, obsType string) {
	t.Helper()
	issued := target.Add(-time.Duration(aggHorizon) * time.Minute)
	collID := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO forecast_collections (id, provider_id, location_id, provider_configuration_id, requested_at, collection_status)
		 VALUES ($1,$2,$3,$4,$5,'success')`,
		collID, catalogdomain.OpenMeteoProviderID, catalogdomain.JohorBahruLocationID, catalogdomain.OpenMeteoConfigID, issued.UTC())
	require.NoError(t, err)
	snapID := ids.New()
	_, err = pool.Exec(ctx,
		`INSERT INTO forecast_snapshots (id, forecast_collection_id, provider_id, location_id, issued_at, target_time,
		   forecast_horizon_minutes, temperature_c, condition_taxonomy_version)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'1')`,
		snapID, collID, catalogdomain.OpenMeteoProviderID, catalogdomain.JohorBahruLocationID,
		issued.UTC(), target.UTC(), aggHorizon, fTemp)
	require.NoError(t, err)
	obsID := ids.New()
	_, err = pool.Exec(ctx,
		`INSERT INTO observations (id, location_id, source, observation_type, observed_at, temperature_c, quality_flag)
		 VALUES ($1,$2,'openmeteo_historical',$3,$4,$5,'valid')`,
		obsID, catalogdomain.JohorBahruLocationID, obsType, target.UTC(), oTemp)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO matched_evaluations (id, forecast_snapshot_id, observation_id, provider_id, location_id,
		   forecast_horizon_minutes, target_time, match_rule, time_delta_minutes)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,'exact_hour',0)`,
		ids.New(), snapID, obsID, catalogdomain.OpenMeteoProviderID, catalogdomain.JohorBahruLocationID, aggHorizon, target.UTC())
	require.NoError(t, err)
}

func newAggregator(pool *pgxpool.Pool) *analysis.AggregateService {
	return analysis.NewAggregateService(analysispg.NewMetricRepository(), dbtx.NewRunner(pool), pool,
		metrics.New(), clock.Real{}, slog_discard())
}

// liveMetric returns the single live (non-superseded) metric row for a cell key.
func liveMetric(ctx context.Context, t *testing.T, pool *pgxpool.Pool, variable, metricType string) (value, ciLo, ciHi *float64, sample int) {
	t.Helper()
	err := pool.QueryRow(ctx,
		`SELECT value, ci_lower, ci_upper, sample_count FROM accuracy_metrics
		 WHERE variable=$1 AND metric_type=$2 AND horizon_minutes=$3 AND superseded_by IS NULL`,
		variable, metricType, aggHorizon).Scan(&value, &ciLo, &ciHi, &sample)
	require.NoError(t, err)
	return value, ciLo, ciHi, sample
}

// TestAggregation_HandComputedAndNull proves the metric aggregation matches the
// TV-1 hand-computed continuous reference and applies the §5 null rule.
func TestAggregation_HandComputedAndNull(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	pool := newPool(ctx, t, connStr)
	(&testEnv{pool: pool}).seedCatalog(ctx, t)

	// TV-1 temperature pairs (f, o): errors 1.5, −1.0, 0.0, 3.0 → mae 1.375,
	// rmse 1.75, bias 0.875. All reanalysis (uniform weight ⇒ cancels).
	day := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -3)
	pairs := [][2]float64{{15.0, 13.5}, {20.0, 21.0}, {18.0, 18.0}, {25.0, 22.0}}
	for i, p := range pairs {
		insertTempPair(ctx, t, pool, day.Add(time.Duration(i)*time.Hour), p[0], p[1], "reanalysis")
	}
	period := analysisdomain.Period{Kind: analysisdomain.PeriodDaily, Start: day, End: day.AddDate(0, 0, 1)}

	agg := newAggregator(pool)
	_, err := agg.AggregatePeriod(ctx, period)
	require.NoError(t, err)

	mae, maeLo, maeHi, n := liveMetric(ctx, t, pool, "temperature", "mae")
	require.NotNil(t, mae)
	assert.InDelta(t, 1.375, *mae, 1e-9)
	assert.Equal(t, 4, n)
	require.NotNil(t, maeLo)
	require.NotNil(t, maeHi)
	assert.LessOrEqual(t, *maeLo, *mae)
	assert.GreaterOrEqual(t, *maeHi, *mae)

	rmse, _, _, _ := liveMetric(ctx, t, pool, "temperature", "rmse")
	require.NotNil(t, rmse)
	assert.InDelta(t, 1.75, *rmse, 1e-9)

	bias, _, _, _ := liveMetric(ctx, t, pool, "temperature", "bias")
	require.NotNil(t, bias)
	assert.InDelta(t, 0.875, *bias, 1e-9)

	// Null rule: humidity was never provided → value NULL, sample_count 0.
	hum, humLo, _, humN := liveMetric(ctx, t, pool, "humidity", "mae")
	assert.Nil(t, hum)
	assert.Nil(t, humLo)
	assert.Equal(t, 0, humN)
}

// TestAggregation_SupersedeAndByteIdentical proves recompute writes new rows,
// supersedes the previous live rows, and yields byte-identical values (property
// 11).
func TestAggregation_SupersedeAndByteIdentical(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	pool := newPool(ctx, t, connStr)
	(&testEnv{pool: pool}).seedCatalog(ctx, t)

	day := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -3)
	for i, p := range [][2]float64{{15.0, 13.5}, {20.0, 21.0}, {18.0, 18.0}, {25.0, 22.0}} {
		insertTempPair(ctx, t, pool, day.Add(time.Duration(i)*time.Hour), p[0], p[1], "reanalysis")
	}
	period := analysisdomain.Period{Kind: analysisdomain.PeriodDaily, Start: day, End: day.AddDate(0, 0, 1)}
	agg := newAggregator(pool)

	_, err := agg.AggregatePeriod(ctx, period)
	require.NoError(t, err)
	first, _, _, _ := liveMetric(ctx, t, pool, "temperature", "mae")
	require.NotNil(t, first)

	// Recompute over the same immutable inputs.
	_, err = agg.AggregatePeriod(ctx, period)
	require.NoError(t, err)

	// Exactly one live temperature-mae row remains; its value is byte-identical.
	second, _, _, _ := liveMetric(ctx, t, pool, "temperature", "mae")
	require.NotNil(t, second)
	assert.Equal(t, *first, *second)

	var live, superseded int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM accuracy_metrics WHERE variable='temperature' AND metric_type='mae' AND superseded_by IS NULL`).Scan(&live))
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM accuracy_metrics WHERE variable='temperature' AND metric_type='mae' AND superseded_by IS NOT NULL`).Scan(&superseded))
	assert.Equal(t, 1, live, "one live row after recompute")
	assert.Equal(t, 1, superseded, "previous row superseded")
}
