package domain

import (
	"time"

	"github.com/google/uuid"
)

// MethodologyVersion stamps every metric row (methodology doc version; §7.5).
const MethodologyVersion = "2026.1"

// Evaluated variables (accuracy_metrics.variable).
const (
	VarTemperature   = "temperature"
	VarWindSpeed     = "wind_speed"
	VarHumidity      = "humidity"
	VarPressure      = "pressure"
	VarPrecipitation = "precipitation"
	VarAll           = "all" // provider-level metrics (reliability)
)

// Metric types (accuracy_metrics.metric_type; methodology §4).
const (
	MetricMAE                 = "mae"
	MetricRMSE                = "rmse"
	MetricBias                = "bias"
	MetricRainMAEAll          = "rain_mae_all"
	MetricRainMAEWet          = "rain_mae_wet"
	MetricRecall              = "recall"
	MetricPrecision           = "precision"
	MetricF1                  = "f1"
	MetricFAR                 = "far"
	MetricThreatScore         = "threat_score"
	MetricOccurrenceAgreement = "occurrence_agreement"
	MetricBrier               = "brier"
	MetricCoverage            = "coverage"
	MetricReliability         = "reliability"
)

// Period kinds (methodology §7.1 / workflow §3).
const (
	PeriodDaily   = "daily"
	PeriodWeekly  = "weekly"
	PeriodMonthly = "monthly"
)

// Cell is an aggregation cell: provider × location × horizon. Metrics are
// produced per (cell × variable × metric_type × period).
type Cell struct {
	ProviderID     uuid.UUID
	LocationID     uuid.UUID
	HorizonMinutes int
}

// Period is a half-open evaluation window [Start, End) with a kind label.
type Period struct {
	Kind  string
	Start time.Time
	End   time.Time
}

// PairRecord holds one matched pair's forecast and observed values for
// aggregation. It represents a live pair (the observation is non-superseded and
// non-suspect; enforced by the read query). Per-variable eligibility is decided
// downstream by eval.Eligible on the individual value pointers.
type PairRecord struct {
	ObservationType string
	QualityFlag     string

	// Forecast side.
	FTemperature *float64
	FWindSpeed   *float64
	FHumidity    *float64
	FPressure    *float64
	FPrecipMM    *float64
	FPrecipProb  *float64

	// Observed side.
	OTemperature *float64
	OWindSpeed   *float64
	OHumidity    *float64
	OPressure    *float64
	OPrecipMM    *float64
}

// AccuracyMetric is one metric row to persist (docs/data/03-table-design.md §4).
// Value/CILower/CIUpper are nil when sample_count = 0 (methodology §5 null rule).
type AccuracyMetric struct {
	ID                 uuid.UUID
	ProviderID         uuid.UUID
	LocationID         uuid.UUID
	HorizonMinutes     int
	Variable           string
	MetricType         string
	Value              *float64
	CILower            *float64
	CIUpper            *float64
	SampleCount        int
	MethodologyVersion string
	PeriodStart        time.Time
	PeriodEnd          time.Time
}
