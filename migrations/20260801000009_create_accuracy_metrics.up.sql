-- 20260801000009_create_accuracy_metrics
-- Analysis module: accuracy_metrics (docs/data/03-table-design.md §4; WP-13).
-- One row per aggregation cell (provider × location × horizon × variable ×
-- metric_type × period). Rows are immutable statistical products; a recompute
-- writes NEW rows and points the previous live row's superseded_by at the new
-- row (the one permitted mutation; workflow §3/§5, methodology §9). value is
-- NULL exactly when sample_count = 0 (methodology §5 null rule).

CREATE TABLE accuracy_metrics (
  id                  uuid PRIMARY KEY,
  provider_id         uuid NOT NULL REFERENCES providers(id),
  location_id         uuid NOT NULL REFERENCES locations(id),
  horizon_minutes     integer NOT NULL,
  variable            text NOT NULL,        -- temperature, precipitation, wind_speed, humidity, pressure, all
  metric_type         text NOT NULL,        -- mae, rmse, bias, rain_mae_all, rain_mae_wet, recall,
                                             -- precision, f1, far, threat_score, occurrence_agreement,
                                             -- brier, coverage, reliability
  value               double precision,     -- NULL ⇔ sample_count = 0
  ci_lower            double precision,
  ci_upper            double precision,
  sample_count        integer NOT NULL DEFAULT 0 CHECK (sample_count >= 0),
  methodology_version text NOT NULL,
  period_start        timestamptz NOT NULL,
  period_end          timestamptz NOT NULL,
  calculated_at       timestamptz NOT NULL DEFAULT now(),
  superseded_by       uuid REFERENCES accuracy_metrics(id),
  CHECK (period_start < period_end),
  CHECK ((value IS NULL) = (sample_count = 0)),
  CHECK (ci_lower IS NULL OR (ci_lower <= value AND value <= ci_upper))
);

-- Latest-row serving + supersede lookup: the live (non-superseded) row per
-- logical cell key. Non-unique: within a recompute tx the new row is inserted
-- (live) before the old row is superseded, so two live rows briefly coexist.
CREATE INDEX accuracy_metrics_live ON accuracy_metrics
  (provider_id, location_id, horizon_minutes, variable, metric_type, period_start, period_end)
  WHERE superseded_by IS NULL;

-- Immutability (methodology §9; domain §2): a metric row changes only by having
-- its superseded_by set once (NULL → value). Everything else — and DELETE — is
-- forbidden.
CREATE OR REPLACE FUNCTION accuracy_metrics_supersede_only() RETURNS trigger AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    RAISE EXCEPTION 'accuracy_metrics rows are immutable (DELETE forbidden)';
  END IF;
  IF OLD.superseded_by IS NOT NULL THEN
    RAISE EXCEPTION 'accuracy_metrics row % already superseded', OLD.id;
  END IF;
  IF NEW.id IS DISTINCT FROM OLD.id
     OR NEW.provider_id IS DISTINCT FROM OLD.provider_id
     OR NEW.location_id IS DISTINCT FROM OLD.location_id
     OR NEW.horizon_minutes IS DISTINCT FROM OLD.horizon_minutes
     OR NEW.variable IS DISTINCT FROM OLD.variable
     OR NEW.metric_type IS DISTINCT FROM OLD.metric_type
     OR NEW.value IS DISTINCT FROM OLD.value
     OR NEW.ci_lower IS DISTINCT FROM OLD.ci_lower
     OR NEW.ci_upper IS DISTINCT FROM OLD.ci_upper
     OR NEW.sample_count IS DISTINCT FROM OLD.sample_count
     OR NEW.methodology_version IS DISTINCT FROM OLD.methodology_version
     OR NEW.period_start IS DISTINCT FROM OLD.period_start
     OR NEW.period_end IS DISTINCT FROM OLD.period_end
     OR NEW.calculated_at IS DISTINCT FROM OLD.calculated_at THEN
    RAISE EXCEPTION 'accuracy_metrics rows are immutable (only superseded_by may be set)';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER accuracy_metrics_supersede_only BEFORE UPDATE OR DELETE ON accuracy_metrics
  FOR EACH ROW EXECUTE FUNCTION accuracy_metrics_supersede_only();
