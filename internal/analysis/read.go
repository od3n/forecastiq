package analysis

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/analysis/domain"
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

// Trends reads the metric buckets for a filter and groups them into per-provider
// series (hollow points preserved: every bucket carries its sample_count).
func (s *ReadService) Trends(ctx context.Context, f ports.TrendFilter) (*TrendsResult, error) {
	buckets, err := s.repo.AccuracyTrends(ctx, s.pool, f)
	if err != nil {
		return nil, err
	}
	byProvider := map[uuid.UUID]*TrendSeries{}
	var order []uuid.UUID
	res := &TrendsResult{HasData: len(buckets) > 0}
	for _, b := range buckets {
		sr, ok := byProvider[b.ProviderID]
		if !ok {
			sr = &TrendSeries{ProviderID: b.ProviderID}
			byProvider[b.ProviderID] = sr
			order = append(order, b.ProviderID)
		}
		sr.Buckets = append(sr.Buckets, b)
		if res.LastPeriodEnd == nil || b.PeriodEnd.After(*res.LastPeriodEnd) {
			pe := b.PeriodEnd
			res.LastPeriodEnd = &pe
		}
	}
	for _, id := range order {
		res.Series = append(res.Series, *byProvider[id])
	}
	return res, nil
}
