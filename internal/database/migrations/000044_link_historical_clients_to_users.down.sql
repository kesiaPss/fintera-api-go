DROP INDEX IF EXISTS idx_historical_sales_client_user_id;

ALTER TABLE historical_sales
    DROP COLUMN IF EXISTS client_user_id;

ALTER TABLE users
    DROP COLUMN IF EXISTS historical_client;
