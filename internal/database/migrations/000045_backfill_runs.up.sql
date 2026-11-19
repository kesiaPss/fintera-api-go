CREATE TABLE IF NOT EXISTS backfill_runs (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(120) NOT NULL,
    version VARCHAR(80) NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'running',
    started_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at TIMESTAMP,
    rows_affected BIGINT NOT NULL DEFAULT 0,
    details TEXT,
    error TEXT
);

CREATE INDEX IF NOT EXISTS idx_backfill_runs_name_version
    ON backfill_runs (name, version);

CREATE INDEX IF NOT EXISTS idx_backfill_runs_status
    ON backfill_runs (status);
