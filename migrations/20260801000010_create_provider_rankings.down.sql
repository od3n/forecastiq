-- 20260801000010_create_provider_rankings (down)
DROP TABLE IF EXISTS provider_rankings CASCADE;
DROP FUNCTION IF EXISTS provider_rankings_supersede_only();
DROP TYPE IF EXISTS ranking_status;
