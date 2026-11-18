DROP INDEX IF EXISTS idx_analytics_cache_global_key;

DROP INDEX IF EXISTS idx_analytics_cache_key_project;
CREATE INDEX IF NOT EXISTS idx_analytics_cache_key_project
    ON analytics_cache(cache_key, project_id);

DROP INDEX IF EXISTS idx_lots_project_status;
DROP INDEX IF EXISTS idx_contracts_status_active_lot;
DROP INDEX IF EXISTS idx_payments_status_payment_date;
DROP INDEX IF EXISTS idx_payments_payment_date;
