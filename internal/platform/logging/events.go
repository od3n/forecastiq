// Package logging — event name registry.
//
// These constants define the stable, alertable log event names emitted by the
// application (observability architecture §2). The event registry is the source
// of truth for log-based queries and alert-rule expressions; changes here must
// be reflected in the Grafana alert rules and log query patterns.
//
// Convention: dot-namespaced, lowercase, past-tense or noun (e.g.
// "collection.completed", "scheduler.slot_missed"). The msg field in every
// structured log line uses one of these constants when the event is stable
// (alertable/queryable). Informal debug lines may use ad-hoc strings.
package logging

// Stable log event names (alertable, queryable). Additions require updating
// the alert rules in deploy/observability/alerts/rules.yaml.
const (
	// Collection pipeline
	EventCollectionStarted      = "collection.started"
	EventCollectionCompleted    = "collection.completed"
	EventCollectionFailed       = "collection.failed"
	EventCollectionDeduplicated = "collection.deduplicated"

	// Observation pipeline
	EventObservationCollected = "observation.collected"
	EventObservationRejected  = "observation.rejected"

	// Analysis engine
	EventMatchingBatchCompleted = "matching.batch_completed"
	EventMetricsBatchCompleted  = "metrics.batch_completed"
	EventRankingsPublished      = "rankings.published"

	// Scheduler
	EventSchedulerSlotClaimed = "scheduler.slot_claimed"
	EventSchedulerSlotMissed  = "scheduler.slot_missed"

	// Provider circuit breaker
	EventCircuitOpened   = "circuit.opened"
	EventCircuitHalfOpen = "circuit.half_open"
	EventCircuitClosed   = "circuit.closed"

	// Storage
	EventPayloadWriteFailed = "payload.write_failed"

	// Data quality
	EventSchemaDriftDetected = "schema_drift.detected"

	// Authentication
	EventAuthLoginFailed = "auth.login_failed"

	// HTTP request (RED summary at handler level)
	EventAPIRequest = "api.request"
)
