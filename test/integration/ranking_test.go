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

const rankHorizon = 1440

var providerX = uuid.MustParse("00000000-0000-0000-0000-0000000000aa")

func fp(x float64) *float64 { return &x }

func insertProviderX(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	// seedCatalog only seeds Open-Meteo; the worked example needs OpenWeather and
	// ProviderX as providers too (accuracy_metrics.provider_id FK).
	insertProvider(ctx, t, pool, catalogdomain.OpenWeatherProviderID, "openweather-test")
	insertProvider(ctx, t, pool, providerX, "providerx")
}

func insertProvider(ctx context.Context, t *testing.T, pool *pgxpool.Pool, id uuid.UUID, slug string) {
	t.Helper()
	_, err := pool.Exec(ctx,
		`INSERT INTO providers (id, name, slug, api_base_url, attribution_text, attribution_url)
		 VALUES ($1,$2,$2,'https://px.example',$2,'https://px.example') ON CONFLICT (id) DO NOTHING`,
		id, slug)
	require.NoError(t, err)
}

func insertAccuracyMetric(ctx context.Context, t *testing.T, pool *pgxpool.Pool, provider uuid.UUID, variable, metricType string, value *float64, sample int, from, to time.Time) {
	t.Helper()
	_, err := pool.Exec(ctx,
		`INSERT INTO accuracy_metrics (id, provider_id, location_id, horizon_minutes, variable, metric_type,
		   value, sample_count, methodology_version, period_start, period_end)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'2026.1',$9,$10)`,
		ids.New(), provider, catalogdomain.JohorBahruLocationID, rankHorizon, variable, metricType,
		value, sample, from.UTC(), to.UTC())
	require.NoError(t, err)
}

// seedWorkedProvider seeds the seven §8 component metrics for one provider.
func seedWorkedProvider(ctx context.Context, t *testing.T, pool *pgxpool.Pool, provider uuid.UUID, tempMAE, bias, f1, rainMAE, windMAE, cov, rel float64, pairs int, from, to time.Time) {
	t.Helper()
	insertAccuracyMetric(ctx, t, pool, provider, "temperature", "mae", fp(tempMAE), pairs, from, to)
	insertAccuracyMetric(ctx, t, pool, provider, "temperature", "bias", fp(bias), pairs, from, to)
	insertAccuracyMetric(ctx, t, pool, provider, "precipitation", "f1", fp(f1), pairs, from, to)
	insertAccuracyMetric(ctx, t, pool, provider, "precipitation", "rain_mae_all", fp(rainMAE), pairs, from, to)
	insertAccuracyMetric(ctx, t, pool, provider, "wind_speed", "mae", fp(windMAE), pairs, from, to)
	insertAccuracyMetric(ctx, t, pool, provider, "temperature", "coverage", fp(cov), pairs, from, to)
	insertAccuracyMetric(ctx, t, pool, provider, "all", "reliability", fp(rel), pairs, from, to)
}

func newRanker(pool *pgxpool.Pool) *analysis.RankService {
	return analysis.NewRankService(analysispg.NewRankingRepository(), dbtx.NewRunner(pool), pool,
		metrics.New(), clock.Real{}, slog_discard())
}

func liveRanking(ctx context.Context, t *testing.T, pool *pgxpool.Pool, provider uuid.UUID) (composite *float64, status string, sample int) {
	t.Helper()
	err := pool.QueryRow(ctx,
		`SELECT composite_score, ranking_status, sample_count FROM provider_rankings
		 WHERE provider_id=$1 AND superseded_by IS NULL`, provider).Scan(&composite, &status, &sample)
	require.NoError(t, err)
	return composite, status, sample
}

// TestRanking_WorkedExample reproduces methodology §8 (Johor Bahru, +24h) end to
// end through the DB. The §8 published OM composite (0.940) contains an
// arithmetic slip; the arithmetically correct value is ≈0.9568 (DR-06). The
// ranking OUTCOME (order + statuses) matches §8 exactly.
func TestRanking_WorkedExample(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	pool := newPool(ctx, t, connStr)
	(&testEnv{pool: pool}).seedCatalog(ctx, t)
	insertProviderX(ctx, t, pool)

	from := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 1, 0)
	seedWorkedProvider(ctx, t, pool, catalogdomain.OpenMeteoProviderID, 1.20, 0.30, 0.769, 0.90, 1.10, 0.98, 0.99, 720, from, to)
	seedWorkedProvider(ctx, t, pool, catalogdomain.OpenWeatherProviderID, 1.50, 0.90, 0.710, 1.40, 1.30, 0.92, 0.97, 700, from, to)
	seedWorkedProvider(ctx, t, pool, providerX, 1.10, 0.25, 0.682, 0.85, 1.60, 0.55, 0.90, 380, from, to)

	ranker := newRanker(pool)
	period := analysisdomain.Period{Kind: analysisdomain.PeriodMonthly, Start: from, End: to}
	n, err := ranker.RankPeriod(ctx, period)
	require.NoError(t, err)
	assert.Equal(t, 3, n)

	omScore, omStatus, omSample := liveRanking(ctx, t, pool, catalogdomain.OpenMeteoProviderID)
	require.NotNil(t, omScore)
	assert.InDelta(t, 0.9568, *omScore, 1e-3)
	assert.Equal(t, "ranked", omStatus)
	assert.Equal(t, 720, omSample)

	owScore, owStatus, _ := liveRanking(ctx, t, pool, catalogdomain.OpenWeatherProviderID)
	require.NotNil(t, owScore)
	assert.InDelta(t, 0.7772, *owScore, 1e-3)
	assert.Equal(t, "ranked", owStatus)

	pxScore, pxStatus, _ := liveRanking(ctx, t, pool, providerX)
	require.NotNil(t, pxScore)
	assert.InDelta(t, 0.6169, *pxScore, 1e-3) // 0.8974 × (0.55/0.8) coverage penalty
	assert.Equal(t, "provisionally_ranked", pxStatus)

	// Order: OM > OW > PX (the §8 ranking outcome).
	assert.Greater(t, *omScore, *owScore)
	assert.Greater(t, *owScore, *pxScore)
}

// TestRanking_SupersedeOnRecompute proves a recompute publishes new rows and
// supersedes the previous live rows (BR-RANK-07).
func TestRanking_SupersedeOnRecompute(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	pool := newPool(ctx, t, connStr)
	(&testEnv{pool: pool}).seedCatalog(ctx, t)

	from := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 1, 0)
	seedWorkedProvider(ctx, t, pool, catalogdomain.OpenMeteoProviderID, 1.20, 0.30, 0.769, 0.90, 1.10, 0.98, 0.99, 720, from, to)
	period := analysisdomain.Period{Kind: analysisdomain.PeriodMonthly, Start: from, End: to}
	ranker := newRanker(pool)

	_, err := ranker.RankPeriod(ctx, period)
	require.NoError(t, err)
	first, _, _ := liveRanking(ctx, t, pool, catalogdomain.OpenMeteoProviderID)
	require.NotNil(t, first)

	_, err = ranker.RankPeriod(ctx, period)
	require.NoError(t, err)
	second, _, _ := liveRanking(ctx, t, pool, catalogdomain.OpenMeteoProviderID)
	require.NotNil(t, second)
	assert.Equal(t, *first, *second) // byte-identical recompute

	var live, superseded int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM provider_rankings WHERE superseded_by IS NULL`).Scan(&live))
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM provider_rankings WHERE superseded_by IS NOT NULL`).Scan(&superseded))
	assert.Equal(t, 1, live)
	assert.Equal(t, 1, superseded)
}
