-- 20260801000007_create_observations (down)
DROP TABLE IF EXISTS observations CASCADE;
DROP FUNCTION IF EXISTS observations_supersede_only();
DROP TYPE IF EXISTS quality_flag;
DROP TYPE IF EXISTS observation_type;
