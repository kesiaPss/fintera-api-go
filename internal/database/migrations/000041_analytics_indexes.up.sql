-- Keep the newest row per cache key and project scope before enforcing
-- uniqueness.
DELETE FROM analytics_cache a
USING analytics_cache b
WHERE a.project_id IS NOT NULL
  AND b.project_id IS NOT NULL
  AND a.cache_key = b.cache_key
  AND a.project_id = b.project_id
  AND a.id < b.id;

DELETE FROM analytics_cache a
USING analytics_cache b
WHERE a.project_id IS NULL
  AND b.project_id IS NULL
  AND a.cache_key = b.cache_key
  AND a.id < b.id;

-- Payment period filters (revenue, averages, collections).
CREATE INDEX IF NOT EXISTS idx_payments_payment_date ON payments(payment_date);
CREATE INDEX IF NOT EXISTS idx_payments_status_payment_date ON payments(status, payment_date);

-- Active-contract counts and occupancy.
CREATE INDEX IF NOT EXISTS idx_contracts_status_active_lot ON contracts(status, active, lot_id);

-- Lot distribution and project occupancy.
CREATE INDEX IF NOT EXISTS idx_lots_project_status ON lots(project_id, status);

-- The pre-existing cache index is not unique, so ON CONFLICT cannot target it.
-- Replace it with a unique index for the per-project upsert.
DROP INDEX IF EXISTS idx_analytics_cache_key_project;
CREATE UNIQUE INDEX IF NOT EXISTS idx_analytics_cache_key_project
    ON analytics_cache(cache_key, project_id);

-- Global cache entries have a NULL project_id, which a regular unique index
-- does not constrain, so use a partial unique index for atomic upserts.
CREATE UNIQUE INDEX IF NOT EXISTS idx_analytics_cache_global_key
    ON analytics_cache(cache_key)
    WHERE project_id IS NULL;
