-- CVT SQLite Schema Migration: Consumer Registry
-- Version: 002

-- Consumers table: stores consumer registrations for contract validation
CREATE TABLE IF NOT EXISTS consumers (
    id                  TEXT PRIMARY KEY,           -- Internal UUID
    consumer_id         TEXT NOT NULL,              -- User-facing consumer ID (e.g., "order-service")
    consumer_version    TEXT NOT NULL,              -- Consumer's version (e.g., "2.1.0")
    schema_id           TEXT NOT NULL,              -- Schema this consumer depends on
    schema_version      TEXT NOT NULL,              -- Schema version consumer was tested against
    environment         TEXT NOT NULL DEFAULT 'dev', -- Environment (dev, staging, prod)
    registered_at       TEXT NOT NULL,              -- ISO8601 timestamp
    last_validated_at   TEXT NOT NULL,              -- ISO8601 timestamp
    used_endpoints      TEXT,                       -- JSON array of EndpointUsage objects

    UNIQUE(consumer_id, schema_id, environment)
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_consumers_consumer_id ON consumers(consumer_id);
CREATE INDEX IF NOT EXISTS idx_consumers_schema_id ON consumers(schema_id);
CREATE INDEX IF NOT EXISTS idx_consumers_environment ON consumers(environment);
CREATE INDEX IF NOT EXISTS idx_consumers_schema_env ON consumers(schema_id, environment);

-- Record this migration
INSERT OR IGNORE INTO migrations (version, applied_at) VALUES (2, datetime('now'));
