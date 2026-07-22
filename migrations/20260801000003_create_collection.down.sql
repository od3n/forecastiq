-- 20260801000003_create_collection (down)
DROP TABLE IF EXISTS forecast_snapshots;          -- cascades to partitions
DROP TABLE IF EXISTS forecast_collections;
DROP FUNCTION IF EXISTS create_monthly_partition(regclass, date);
DROP FUNCTION IF EXISTS forecast_collections_immutable();
