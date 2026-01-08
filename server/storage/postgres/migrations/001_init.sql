-- CVT PostgreSQL Schema Migration: Initial Setup
-- Version: 001

-- Schemas table: stores OpenAPI schemas with versioning
CREATE TABLE IF NOT EXISTS schemas (
    id              UUID PRIMARY KEY,
    schema_id       TEXT NOT NULL,
    version         TEXT NOT NULL,
    content         TEXT NOT NULL,
    content_hash    TEXT NOT NULL,
    openapi_version TEXT NOT NULL,
    endpoint_count  INTEGER NOT NULL DEFAULT 0,
    is_latest       BOOLEAN NOT NULL DEFAULT FALSE,
    registered_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Ownership fields
    owner           TEXT,
    team            TEXT,
    contact_email   TEXT,
    read_only       BOOLEAN NOT NULL DEFAULT FALSE,

    -- Environment tagging
    environment     TEXT NOT NULL DEFAULT 'dev',

    UNIQUE(schema_id, version)
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_schemas_schema_id ON schemas(schema_id);
CREATE INDEX IF NOT EXISTS idx_schemas_schema_id_latest ON schemas(schema_id) WHERE is_latest = TRUE;
CREATE INDEX IF NOT EXISTS idx_schemas_environment ON schemas(environment);
CREATE INDEX IF NOT EXISTS idx_schemas_owner ON schemas(owner);
CREATE INDEX IF NOT EXISTS idx_schemas_team ON schemas(team);
CREATE INDEX IF NOT EXISTS idx_schemas_registered_at ON schemas(registered_at);

-- Validation runs table: stores validation history for analytics
CREATE TABLE IF NOT EXISTS validation_runs (
    id              UUID PRIMARY KEY,
    schema_id       TEXT NOT NULL,
    schema_version  TEXT NOT NULL,
    schema_hash     TEXT NOT NULL,

    -- Request details
    request_method  TEXT NOT NULL,
    request_path    TEXT NOT NULL,
    request_headers JSONB,
    request_body    TEXT,

    -- Response details
    response_status INTEGER NOT NULL,
    response_headers JSONB,
    response_body   TEXT,

    -- Validation result
    valid           BOOLEAN NOT NULL,
    errors          JSONB,

    -- Timing and context
    duration_ms     BIGINT NOT NULL,
    validated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    environment     TEXT NOT NULL DEFAULT 'dev',
    client_id       TEXT,
    client_ip       TEXT
);

-- Indexes for analytics queries
CREATE INDEX IF NOT EXISTS idx_validation_runs_schema_id ON validation_runs(schema_id);
CREATE INDEX IF NOT EXISTS idx_validation_runs_validated_at ON validation_runs(validated_at);
CREATE INDEX IF NOT EXISTS idx_validation_runs_environment ON validation_runs(environment);
CREATE INDEX IF NOT EXISTS idx_validation_runs_valid ON validation_runs(valid);
CREATE INDEX IF NOT EXISTS idx_validation_runs_schema_method ON validation_runs(schema_id, request_method);

-- GIN index for JSONB error querying
CREATE INDEX IF NOT EXISTS idx_validation_runs_errors_gin ON validation_runs USING GIN (errors);

-- Schema comparisons table: stores breaking change analysis results
CREATE TABLE IF NOT EXISTS schema_comparisons (
    id              UUID PRIMARY KEY,
    schema_id       TEXT NOT NULL,
    old_version     TEXT NOT NULL,
    new_version     TEXT NOT NULL,
    compatible      BOOLEAN NOT NULL,
    breaking_changes JSONB NOT NULL,
    compared_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(schema_id, old_version, new_version)
);

CREATE INDEX IF NOT EXISTS idx_schema_comparisons_schema_id ON schema_comparisons(schema_id);
CREATE INDEX IF NOT EXISTS idx_schema_comparisons_compared_at ON schema_comparisons(compared_at);

-- Migration tracking table
CREATE TABLE IF NOT EXISTS migrations (
    version     INTEGER PRIMARY KEY,
    applied_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Record this migration
INSERT INTO migrations (version) VALUES (1) ON CONFLICT DO NOTHING;
