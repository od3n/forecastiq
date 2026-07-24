package analysis

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/analysis/domain"
	"github.com/forecastiq/forecastiq/internal/analysis/eval"
	"github.com/forecastiq/forecastiq/internal/analysis/ports"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
)

// DefaultRankingHorizon is the horizon served when a request omits
// horizon_minutes (+24h — the S-01 primary planning horizon and the §8 worked
// example). Cross-horizon profile composites remain a documented follow-on.
const DefaultRankingHorizon = 1440

// DefaultMinSampleCount echoes the methodology §7.1 ranked threshold.
const DefaultMinSampleCount = 30

// RankingsQuery selects a ranking cohort (GET /rankings).
type RankingsQuery struct {
	LocationID     uuid.UUID
	HorizonMinutes int
	Profile        string
	MinSampleCount int
}

// RankedRow is one provider's ranking row with its resolved display rank.
// Rank 0 means unscored (unranked status): shown as "insufficient data", never
// ordered. Tied is true when the row shares its rank via CI overlap (§7.4).
type RankedRow struct {
	Row  *ports.RankingReadRow
	Rank int
	Tied bool
}

// RankingsResult is the assembled GET /rankings payload (pre-DTO / pre-round).
type RankingsResult struct {
	Rows               []RankedRow
	Observation        *ports.ObservationContextRow
	MethodologyVersion string
	WeightsVersion     string
	HorizonMinutes     int
	HorizonProfile     string
	MinSampleCount     int
	PeriodStart        time.Time
	PeriodEnd          time.Time
	LastCalculatedAt   *time.Time
	HasRows            bool
}

// ReadService serves the public dashboard reads over pre-computed analysis rows
// (WP-15). It owns no writes; ordering + tie grouping reuse the domain engine so
// the served rank matches the methodology exactly.
type ReadService struct {
	repo ports.ReadRepository
	pool dbtx.DBTX
}

// NewReadService wires a ReadService.
func NewReadService(repo ports.ReadRepository, pool dbtx.DBTX) *ReadService {
	return &ReadService{repo: repo, pool: pool}
}

// Methodology returns the published methodology document (GET /rankings/methodology).
func (s *ReadService) Methodology() domain.MethodologyDoc { return domain.Methodology() }

// Rankings assembles the ranking cohort for a location + horizon + profile:
// the live stored rows, ordered (ranked → provisional → unranked) with CI-overlap
// tie grouping, plus the latest observation context.
func (s *ReadService) Rankings(ctx context.Context, q RankingsQuery) (*RankingsResult, error) {
	if q.Profile == "" {
		q.Profile = domain.ProfileUniform
	}
	if q.MinSampleCount <= 0 {
		q.MinSampleCount = DefaultMinSampleCount
	}
	rows, err := s.repo.ListRankings(ctx, s.pool, q.LocationID, q.HorizonMinutes, q.Profile)
	if err != nil {
		return nil, err
	}
	obs, err := s.repo.LatestObservation(ctx, s.pool, q.LocationID)
	if err != nil {
		return nil, err
	}

	res := &RankingsResult{
		MethodologyVersion: domain.MethodologyVersion,
		WeightsVersion:     domain.WeightsVersion,
		HorizonMinutes:     q.HorizonMinutes,
		HorizonProfile:     q.Profile,
		MinSampleCount:     q.MinSampleCount,
		Observation:        obs,
		HasRows:            len(rows) > 0,
	}
	if len(rows) == 0 {
		return res, nil
	}
	res.PeriodStart = rows[0].PeriodStart
	res.PeriodEnd = rows[0].PeriodEnd
	last := rows[0].CalculatedAt
	for _, r := range rows[1:] {
		if r.CalculatedAt.After(last) {
			last = r.CalculatedAt
		}
	}
	res.LastCalculatedAt = &last
	res.Rows = orderRankings(rows)
	return res, nil
}

// orderRankings assigns display ranks: `ranked` providers first (composite
// desc, CI-overlap tie groups share a rank number), then `provisionally_ranked`
// (continuing the numbering), then `unranked` rows (rank 0, unordered). Reuses
// domain.RankOrder for the CI-overlap grouping so ties match the methodology.
func orderRankings(rows []*ports.RankingReadRow) []RankedRow {
	var ranked, provisional, unranked []*ports.RankingReadRow
	for _, r := range rows {
		switch r.Status {
		case domain.StatusRanked:
			ranked = append(ranked, r)
		case domain.StatusProvisionally:
			provisional = append(provisional, r)
		default:
			unranked = append(unranked, r)
		}
	}
	out := make([]RankedRow, 0, len(rows))
	next := rankGroups(ranked, 1, &out)
	rankGroups(provisional, next, &out)
	for _, r := range unranked {
		out = append(out, RankedRow{Row: r, Rank: 0})
	}
	return out
}

// rankGroups appends CI-overlap-grouped rows to out starting at base and
// returns the next free rank number.
func rankGroups(subset []*ports.RankingReadRow, base int, out *[]RankedRow) int {
	if len(subset) == 0 {
		return base
	}
	prs := make([]domain.ProviderRanking, len(subset))
	for i, r := range subset {
		prs[i] = domain.ProviderRanking{
			ProviderID:     r.ProviderID,
			CompositeScore: r.CompositeScore,
			CILower:        r.CILower,
			CIUpper:        r.CIUpper,
		}
	}
	groups := domain.RankOrder(prs) // groups of indexes into prs/subset, best first
	rank := base
	for _, g := range groups {
		tied := len(g) > 1
		for _, idx := range g {
			*out = append(*out, RankedRow{Row: subset[idx], Rank: rank, Tied: tied})
		}
		rank++
	}
	return rank
}

// ── Accuracy summary (S-02 location mode / S-03 provider mode) ──────────

// LocationSummaryProvider is one provider's metric grid + status + window at a
// location, grouped for the S-02 detail screen. Provider identity is resolved
// by the API layer (catalog), keeping this read free of catalog concerns.
type LocationSummaryProvider struct {
	ProviderID    uuid.UUID
	RankingStatus string
	Metrics       []*ports.MetricRow
	Window        ports.CollectionWindow
}

// LocationSummary is the assembled S-02 payload.
type LocationSummary struct {
	HorizonMinutes int
	Providers      []LocationSummaryProvider
	LastSnapshotAt *time.Time
	HasData        bool
}

// ProviderSummary is the assembled S-03 payload (a provider's ranking cells).
type ProviderSummary struct {
	Cells   []*ports.ProviderSummaryCell
	Windows map[uuid.UUID]ports.CollectionWindow
	HasData bool
}

// LocationSummary assembles the per-provider metric grid, ranking status, and
// collection window for a location + horizon (S-02).
func (s *ReadService) LocationSummary(ctx context.Context, locationID uuid.UUID, horizonMinutes int) (*LocationSummary, error) {
	metrics, err := s.repo.LocationMetrics(ctx, s.pool, locationID, horizonMinutes)
	if err != nil {
		return nil, err
	}
	statuses, err := s.repo.LocationProviderStatuses(ctx, s.pool, locationID, horizonMinutes)
	if err != nil {
		return nil, err
	}
	windows, err := s.repo.LocationWindows(ctx, s.pool, locationID)
	if err != nil {
		return nil, err
	}

	// Group metrics by provider, preserving first-seen order (query is ordered
	// by provider_id).
	byProvider := map[uuid.UUID]*LocationSummaryProvider{}
	var order []uuid.UUID
	for _, m := range metrics {
		p, ok := byProvider[m.ProviderID]
		if !ok {
			st := statuses[m.ProviderID]
			w := windows[m.ProviderID]
			w.Coverage, w.Reliability = st.Coverage, st.Reliability
			p = &LocationSummaryProvider{ProviderID: m.ProviderID, RankingStatus: st.RankingStatus, Window: w}
			byProvider[m.ProviderID] = p
			order = append(order, m.ProviderID)
		}
		p.Metrics = append(p.Metrics, m)
	}

	res := &LocationSummary{HorizonMinutes: horizonMinutes, HasData: len(order) > 0}
	for _, id := range order {
		p := byProvider[id]
		if p.Window.LastSnapshotAt != nil && (res.LastSnapshotAt == nil || p.Window.LastSnapshotAt.After(*res.LastSnapshotAt)) {
			res.LastSnapshotAt = p.Window.LastSnapshotAt
		}
		res.Providers = append(res.Providers, *p)
	}
	return res, nil
}

// ProviderSummary assembles a provider's ranking cells across locations +
// horizons with per-location collection windows (S-03).
func (s *ReadService) ProviderSummary(ctx context.Context, providerID uuid.UUID) (*ProviderSummary, error) {
	cells, err := s.repo.ProviderRankingCells(ctx, s.pool, providerID)
	if err != nil {
		return nil, err
	}
	windows, err := s.repo.ProviderWindows(ctx, s.pool, providerID)
	if err != nil {
		return nil, err
	}
	return &ProviderSummary{Cells: cells, Windows: windows, HasData: len(cells) > 0}, nil
}

// ── Accuracy trends (S-04 / S-03 per-horizon detail) ────────────────────

// TrendSeries is one provider's ordered metric buckets.
type TrendSeries struct {
	ProviderID uuid.UUID
	Buckets    []*ports.TrendBucket
}

// TrendsResult is the assembled GET /accuracy payload.
type TrendsResult struct {
	Series        []TrendSeries
	HasData       bool
	LastPeriodEnd *time.Time
}

// Trends reads the metric span-rows for a filter and groups them into per-
// provider series, re-bucketed into the requested timezone (WP-17): buckets are
// aligned to tz-local day/week/month boundaries — the presentation-layer
// equivalent of date_trunc(granularity, period_start AT TIME ZONE tz), applied
// post-scan on ≤ 365 rows (query doc §bucketing). Metric values remain the
// UTC-computed aggregates (methodology §3.1 matches in UTC); the tz alignment is
// display-only (BR-TZ-02..05). Where a tz bucket spans multiple stored rows the
// values combine by sample-count-weighted mean (CI dropped); the common daily
// case is a 1:1 relabel that preserves value + CI. Hollow points (sample_count
// 0, value nil) are preserved.
func (s *ReadService) Trends(ctx context.Context, f ports.TrendFilter, loc *time.Location) (*TrendsResult, error) {
	if loc == nil {
		loc = time.UTC
	}
	rows, err := s.repo.AccuracyTrends(ctx, s.pool, f)
	if err != nil {
		return nil, err
	}
	byProvider := map[uuid.UUID]*trendGrouping{}
	var order []uuid.UUID
	res := &TrendsResult{HasData: len(rows) > 0}
	for _, b := range rows {
		g, ok := byProvider[b.ProviderID]
		if !ok {
			g = newTrendGrouping()
			byProvider[b.ProviderID] = g
			order = append(order, b.ProviderID)
		}
		g.add(b, loc, f.Aggregation)
	}
	for _, id := range order {
		buckets := byProvider[id].buckets()
		res.Series = append(res.Series, TrendSeries{ProviderID: id, Buckets: buckets})
		if n := len(buckets); n > 0 {
			if end := buckets[n-1].PeriodEnd; res.LastPeriodEnd == nil || end.After(*res.LastPeriodEnd) {
				pe := end
				res.LastPeriodEnd = &pe
			}
		}
	}
	return res, nil
}

// trendBucketAcc accumulates the stored rows that fall in one tz bucket.
type trendBucketAcc struct {
	start  time.Time
	end    time.Time
	sumVW  float64 // Σ value·sample_count (sample-count-weighted)
	sumW   float64 // Σ sample_count over rows with a non-null value
	sample int
	n      int
	single *ports.TrendBucket
}

// trendGrouping re-buckets one provider's stored rows into tz-aligned buckets.
type trendGrouping struct {
	by    map[int64]*trendBucketAcc
	order []int64
}

func newTrendGrouping() *trendGrouping {
	return &trendGrouping{by: map[int64]*trendBucketAcc{}}
}

func (g *trendGrouping) add(b *ports.TrendBucket, loc *time.Location, aggregation string) {
	start, end := truncToBucket(b.PeriodStart, loc, aggregation)
	key := start.UnixNano()
	a, ok := g.by[key]
	if !ok {
		a = &trendBucketAcc{start: start, end: end}
		g.by[key] = a
		g.order = append(g.order, key)
	}
	a.n++
	a.single = b
	a.sample += b.SampleCount
	if b.Value != nil {
		w := float64(b.SampleCount)
		if w == 0 {
			w = 1 // a valued row with zero sample count still contributes its value
		}
		a.sumVW += *b.Value * w
		a.sumW += w
	}
}

func (g *trendGrouping) buckets() []*ports.TrendBucket {
	sort.Slice(g.order, func(i, j int) bool { return g.order[i] < g.order[j] })
	out := make([]*ports.TrendBucket, 0, len(g.order))
	for _, key := range g.order {
		a := g.by[key]
		b := &ports.TrendBucket{
			ProviderID:  a.single.ProviderID,
			PeriodStart: a.start.UTC(),
			PeriodEnd:   a.end.UTC(),
			SampleCount: a.sample,
		}
		if a.n == 1 {
			// 1:1 relabel — preserve the stored value + CI exactly.
			b.Value, b.CILower, b.CIUpper = a.single.Value, a.single.CILower, a.single.CIUpper
		} else if a.sumW > 0 {
			v := a.sumVW / a.sumW
			b.Value = &v // combined buckets drop CI (not recoverable from row values)
		}
		out = append(out, b)
	}
	return out
}

// truncToBucket returns the tz-local bucket [start, end) containing t for the
// aggregation granularity (daily/weekly/monthly). Week starts Monday. DST and
// variable month length are handled by time.AddDate normalization.
func truncToBucket(t time.Time, loc *time.Location, aggregation string) (start, end time.Time) {
	lt := t.In(loc)
	switch aggregation {
	case "weekly":
		back := (int(lt.Weekday()) + 6) % 7 // days since Monday
		start = time.Date(lt.Year(), lt.Month(), lt.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -back)
		end = start.AddDate(0, 0, 7)
	case "monthly":
		start = time.Date(lt.Year(), lt.Month(), 1, 0, 0, 0, 0, loc)
		end = start.AddDate(0, 1, 0)
	default: // daily
		start = time.Date(lt.Year(), lt.Month(), lt.Day(), 0, 0, 0, 0, loc)
		end = start.AddDate(0, 0, 1)
	}
	return start, end
}

// ── Forecast-vs-Actual (S-05 forecast-comparison) ──────────────────────

// ComparisonQuery selects the FvA payload (GET /forecast-comparison). The day
// window [From, To) is the requested date interpreted in the location timezone,
// resolved to UTC by the API layer.
type ComparisonQuery struct {
	LocationID     uuid.UUID
	ProviderIDs    []uuid.UUID
	Variable       string
	HorizonMinutes int
	From           time.Time
	To             time.Time
}

// ComparisonSeries is one provider's forecast line for the day. IssuedAt is the
// earliest point's actual issuance (representative; per-point issuance rides on
// each point for nearest-shorter honesty).
type ComparisonSeries struct {
	ProviderID uuid.UUID
	IssuedAt   time.Time
	Points     []*ports.ForecastPoint
}

// ComparisonDayMetric is one provider's in-memory day accuracy (≤ 24 pairs),
// computed with the WP-12 evaluation kernel under observation-quality weights.
type ComparisonDayMetric struct {
	ProviderID  uuid.UUID
	MAE         *float64
	RMSE        *float64
	Bias        *float64
	SampleCount int
}

// ComparisonResult is the assembled GET /forecast-comparison payload.
type ComparisonResult struct {
	Series                []ComparisonSeries
	Observations          []*ports.ComparisonObservation
	DayMetrics            []ComparisonDayMetric
	ErrorBandMAE          *float64
	ObservationsAvailable bool
	ProvenanceMix         map[string]float64
	LatestObservedAt      *time.Time
}

// ForecastComparison assembles the S-05 payload: per-provider forecast lines at
// the DR-02-selected issuance, the day's observations (gaps absent), and
// in-memory day metrics computed against those observations with the WP-12
// kernel. The error band is the pooled MAE across all matched pairs.
func (s *ReadService) ForecastComparison(ctx context.Context, q ComparisonQuery) (*ComparisonResult, error) {
	points, err := s.repo.ForecastComparisonPoints(ctx, s.pool, q.LocationID, q.ProviderIDs, q.Variable, q.HorizonMinutes, q.From, q.To)
	if err != nil {
		return nil, err
	}
	obs, err := s.repo.ComparisonObservations(ctx, s.pool, q.LocationID, q.Variable, q.From, q.To)
	if err != nil {
		return nil, err
	}

	obsByHour := make(map[time.Time]*ports.ComparisonObservation, len(obs))
	for _, o := range obs {
		obsByHour[o.ObservedAt.UTC()] = o
	}

	type seriesAcc struct {
		series *ComparisonSeries
		cont   eval.Continuous
	}
	accs := map[uuid.UUID]*seriesAcc{}
	var pooled eval.Continuous
	for _, p := range points {
		a, ok := accs[p.ProviderID]
		if !ok {
			a = &seriesAcc{series: &ComparisonSeries{ProviderID: p.ProviderID, IssuedAt: p.IssuedAt}}
			accs[p.ProviderID] = a
		}
		a.series.Points = append(a.series.Points, p)
		if p.IssuedAt.Before(a.series.IssuedAt) {
			a.series.IssuedAt = p.IssuedAt
		}
		if o, matched := obsByHour[p.TargetTime.UTC()]; matched {
			w := eval.ProvenanceWeight(o.ObservationType)
			a.cont.Add(p.Value, o.Value, w)
			pooled.Add(p.Value, o.Value, w)
		}
	}

	res := &ComparisonResult{
		Observations:          obs,
		ObservationsAvailable: len(obs) > 0,
		ErrorBandMAE:          pooled.MAE(),
		ProvenanceMix:         provenanceMix(obs),
	}
	// Emit series + day metrics in the requested provider order (deterministic).
	for _, id := range q.ProviderIDs {
		a, ok := accs[id]
		if !ok {
			continue
		}
		res.Series = append(res.Series, *a.series)
		res.DayMetrics = append(res.DayMetrics, ComparisonDayMetric{
			ProviderID: id, MAE: a.cont.MAE(), RMSE: a.cont.RMSE(), Bias: a.cont.Bias(), SampleCount: a.cont.N(),
		})
	}
	if len(obs) > 0 {
		last := obs[len(obs)-1].ObservedAt.UTC()
		res.LatestObservedAt = &last
	}
	return res, nil
}

// provenanceMix returns the fraction of observations by observation_type.
func provenanceMix(obs []*ports.ComparisonObservation) map[string]float64 {
	if len(obs) == 0 {
		return nil
	}
	counts := map[string]int{}
	for _, o := range obs {
		counts[o.ObservationType]++
	}
	mix := make(map[string]float64, len(counts))
	for k, v := range counts {
		mix[k] = float64(v) / float64(len(obs))
	}
	return mix
}
