-- 20260801000013_add_metrics_calculated_index
-- WP-26b (PT-6 baseline finding): docs/data/04-index-and-query-plan.md §1.5
-- specifies `metrics_calculated (calculated_at DESC)` serving the engine-lag
-- query (`SELECT max(calculated_at) FROM accuracy_metrics WHERE superseded_by
-- IS NULL`, adminpg), but the index was never created by migration
-- 20260801000009. At base perf volume (~150K metric rows) the query seq-scans
-- (~140 ms measured), breaching the S-10 health-assembly p95 < 200 ms budget
-- under polling (PT-6). A backward scan of this index finds the max
-- immediately (the newest rows are the live ones after a recompute).
CREATE INDEX metrics_calculated ON accuracy_metrics (calculated_at DESC);
