-- CVT PostgreSQL Schema Migration: Consumer Registry
-- Version: 002

-- Consumers table: stores consumer registrations for contract validation
CREATE TABLE IF NOT EXISTS consumers (
    id                  UUID PRIMARY KEY,
    consumer_id         VARCHAR(255) NOT NULL,
    consumer_version    VARCHAR(100) NOT NULL,
    schema_id           VARCHAR(255) NOT NULL,
    schema_version      VARCHAR(100) NOT NULL,
    environment         VARCHAR(50) NOT NULL DEFAULT 'dev',
    registered_at       TIMESTAMP WITH TIME ZONE NOT NULL,
    last_validated_at   TIMESTAMP WITH TIME ZONE NOT NULL,
    used_endpoints      JSONB,

    UNIQUE(consumer_id, schema_id, environment)
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_consumers_consumer_id ON consumers(consumer_id);
CREATE INDEX IF NOT EXISTS idx_consumers_schema_id ON consumers(schema_id);
CREATE INDEX IF NOT EXISTS idx_consumers_environment ON consumers(environment);
CREATE INDEX IF NOT EXISTS idx_consumers_schema_env ON consumers(schema_id, environment);

-- Record this migration
INSERT INTO migrations (version, applied_at)
VALUES (2, NOW())
ON CONFLICT (version) DO NOTHING;
