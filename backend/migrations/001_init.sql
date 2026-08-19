CREATE TABLE IF NOT EXISTS rate_snapshots (
    id BIGSERIAL PRIMARY KEY,
    provider TEXT NOT NULL,
    currency CHAR(3) NOT NULL,
    buy_rate NUMERIC(20, 8) NOT NULL,
    sell_rate NUMERIC(20, 8) NOT NULL,
    fee_percent NUMERIC(8, 4) NOT NULL DEFAULT 0,
    source_url TEXT NOT NULL,
    effective_at TIMESTAMPTZ NOT NULL,
    fetched_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT rate_snapshots_positive_rates CHECK (buy_rate > 0 AND sell_rate > 0)
);

CREATE INDEX IF NOT EXISTS rate_snapshots_latest_idx
    ON rate_snapshots (provider, currency, fetched_at DESC);

CREATE INDEX IF NOT EXISTS rate_snapshots_history_idx
    ON rate_snapshots (provider, currency, fetched_at);

CREATE TABLE IF NOT EXISTS collection_runs (
    id BIGSERIAL PRIMARY KEY,
    provider TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('success', 'failed')),
    message TEXT,
    started_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ NOT NULL
);

-- Transilvania Exchange was removed when the product focus moved to Bucharest.
-- Keep existing installations free of snapshots collected by the retired adapter.
DELETE FROM rate_snapshots
WHERE provider = 'transilvania_exchange';

DELETE FROM collection_runs
WHERE provider = 'transilvania_exchange';
