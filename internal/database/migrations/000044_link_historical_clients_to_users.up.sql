-- Link migrated historical clients to a real customer account. The flag lets the
-- admin Users list identify accounts created only to represent a historical
-- client, and the foreign key lets the reservation flow reuse the same person.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS historical_client BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE historical_sales
    ADD COLUMN IF NOT EXISTS client_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_historical_sales_client_user_id
    ON historical_sales (client_user_id);
