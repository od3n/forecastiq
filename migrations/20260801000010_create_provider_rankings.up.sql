-- 20260801000010_create_provider_rankings
-- Analysis module: provider_rankings (docs/data/03-table-design.md §4; WP-14).
-- One row per ranking cell (provider × location × horizon × profile × period).
-- Composite scores are immutable statistical products; a recompute writes NEW
-- rows and points the previous live row's superseded_by at the new row (the one
-- permitted mutation; workflow §4/§5, methodology §7.5, BR-RANK-07).
-- The ranking_status enum lands with this table (deferred from migration 1).

CREATE TYPE ranking_status AS ENUM ('ranked', 'provisionally_ranked', 'unranked');

CREATE TABLE provider_rankings (
  id                  uuid PRIMARY KEY,
  provider_id         uuid NOT NULL REFERENCES providers(id),
  location_id         uuid NOT NULL REFERENCES locations(id),
  horizon_minutes     integer NOT NULL,
  composite_score     double precision,    -- NULL ⇔ unranked
  ci_lower            double precision,
  ci_upper            double precision,
  ranking_status      ranking_status NOT NULL,
  sample_count        integer NOT NULL DEFAULT 0 CHECK (sample_count >= 0),
  coverage            double precision CHECK (coverage BETWEEN 0 AND 1),
  reliability         double precision CHECK (reliability BETWEEN 0 AND 1),
  component_scores    jsonb NOT NULL,
  methodology_version text NOT NULL,
  weights_version     text NOT NULL,
  horizon_profile     text NOT NULL DEFAULT 'uniform',
  period_start        timestamptz NOT NULL,
  period_end          timestamptz NOT NULL,
  calculated_at       timestamptz NOT NULL DEFAULT now(),
  superseded_by       uuid REFERENCES provider_rankings(id),
  CHECK (period_start < period_end),
  CHECK ((composite_score IS NULL) = (ranking_status = 'unranked')),
  CHECK (composite_score IS NULL OR composite_score BETWEEN 0 AND 1)
);

-- Publication surface: latest (non-superseded) rows per logical key
-- (workflow §6). Non-unique: within a recompute tx the new row is inserted
-- (live) before the old row is superseded, so two live rows briefly coexist.
CREATE INDEX provider_rankings_live ON provider_rankings
  (location_id, horizon_minutes, horizon_profile, period_start, period_end)
  WHERE superseded_by IS NULL;

-- Immutability (BR-RANK-07): a ranking row changes only by having superseded_by
-- set once (NULL → value). Everything else — and DELETE — is forbidden.
CREATE OR REPLACE FUNCTION provider_rankings_supersede_only() RETURNS trigger AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    RAISE EXCEPTION 'provider_rankings rows are immutable (DELETE forbidden)';
  END IF;
  IF OLD.superseded_by IS NOT NULL THEN
    RAISE EXCEPTION 'provider_rankings row % already superseded', OLD.id;
  END IF;
  IF NEW.id IS DISTINCT FROM OLD.id
     OR NEW.provider_id IS DISTINCT FROM OLD.provider_id
     OR NEW.location_id IS DISTINCT FROM OLD.location_id
     OR NEW.horizon_minutes IS DISTINCT FROM OLD.horizon_minutes
     OR NEW.composite_score IS DISTINCT FROM OLD.composite_score
     OR NEW.ci_lower IS DISTINCT FROM OLD.ci_lower
     OR NEW.ci_upper IS DISTINCT FROM OLD.ci_upper
     OR NEW.ranking_status IS DISTINCT FROM OLD.ranking_status
     OR NEW.sample_count IS DISTINCT FROM OLD.sample_count
     OR NEW.coverage IS DISTINCT FROM OLD.coverage
     OR NEW.reliability IS DISTINCT FROM OLD.reliability
     OR NEW.component_scores IS DISTINCT FROM OLD.component_scores
     OR NEW.methodology_version IS DISTINCT FROM OLD.methodology_version
     OR NEW.weights_version IS DISTINCT FROM OLD.weights_version
     OR NEW.horizon_profile IS DISTINCT FROM OLD.horizon_profile
     OR NEW.period_start IS DISTINCT FROM OLD.period_start
     OR NEW.period_end IS DISTINCT FROM OLD.period_end
     OR NEW.calculated_at IS DISTINCT FROM OLD.calculated_at THEN
    RAISE EXCEPTION 'provider_rankings rows are immutable (only superseded_by may be set)';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER provider_rankings_supersede_only BEFORE UPDATE OR DELETE ON provider_rankings
  FOR EACH ROW EXECUTE FUNCTION provider_rankings_supersede_only();
