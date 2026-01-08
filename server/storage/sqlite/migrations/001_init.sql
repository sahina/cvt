-- CVT SQLite Schema Migration: Initial Setup
-- Version: 001

-- Enable WAL mode for better concurrent performance
PRAGMA journal_mode=WAL;

-- Schemas table: stores OpenAPI schemas with versioning
CREATE TABLE IF NOT EXISTS schemas (
    id              TEXT PRIMARY KEY,           -- Internal UUID
    schema_id       TEXT NOT NULL,              -- User-facing schema ID (e.g., "user-service")
    version         TEXT NOT NULL,              -- Semantic version (e.g., "1.2.3")
    content         TEXT NOT NULL,              -- Original OpenAPI schema content (JSON/YAML)
    content_hash    TEXT NOT NULL,              -- SHA256 hash of normalized content
    openapi_version TEXT NOT NULL,              -- Detected version (e.g., "3.0.0", "2.0")
    endpoint_count  INTEGER NOT NULL DEFAULT 0, -- Number of endpoints in schema
    is_latest       INTEGER NOT NULL DEFAULT 0, -- 1 if this is the latest version
    registered_at   TEXT NOT NULL,              -- ISO8601 timestamp
    updated_at      TEXT NOT NULL,              -- ISO8601 timestamp

    -- Ownership fields
    owner           TEXT,                       -- Owner name/identifier
    team            TEXT,                       -- Team responsible
    contact_email   TEXT,                       -- Contact email
    read_only       INTEGER NOT NULL DEFAULT 0, -- Immutable flag (1=true)

    -- Environment tagging
    environment     TEXT NOT NULL DEFAULT 'dev', -- dev, staging, prod

    UNIQUE(schema_id, version)
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_schemas_schema_id ON schemas(schema_id);
CREATE INDEX IF NOT EXISTS idx_schemas_schema_id_latest ON schemas(schema_id, is_latest);
CREATE INDEX IF NOT EXISTS idx_schemas_environment ON schemas(environment);
CREATE INDEX IF NOT EXISTS idx_schemas_owner ON schemas(owner);
CREATE INDEX IF NOT EXISTS idx_schemas_team ON schemas(team);
CREATE INDEX IF NOT EXISTS idx_schemas_registered_at ON schemas(registered_at);

-- Validation runs table: stores validation history for analytics
CREATE TABLE IF NOT EXISTS validation_runs (
    id              TEXT PRIMARY KEY,           -- UUID
    schema_id       TEXT NOT NULL,              -- References schemas.schema_id
    schema_version  TEXT NOT NULL,              -- Version used for validation
    schema_hash     TEXT NOT NULL,              -- Hash of schema used

    -- Request details
    request_method  TEXT NOT NULL,              -- HTTP method (GET, POST, etc.)
    request_path    TEXT NOT NULL,              -- API path validated
    request_headers TEXT,                       -- JSON-encoded headers
    request_body    TEXT,                       -- Request body

    -- Response details
    response_status INTEGER NOT NULL,           -- HTTP status code
    response_headers TEXT,                      -- JSON-encoded headers
    response_body   TEXT,                       -- Response body

    -- Validation result
    valid           INTEGER NOT NULL,           -- 1 = valid, 0 = invalid
    errors          TEXT,                       -- JSON array of error strings

    -- Timing and context
    duration_ms     INTEGER NOT NULL,           -- Validation duration in milliseconds
    validated_at    TEXT NOT NULL,              -- ISO8601 timestamp
    environment     TEXT NOT NULL DEFAULT 'dev',
    client_id       TEXT,                       -- API key ID if authenticated
    client_ip       TEXT                        -- Client IP address
);

-- Indexes for analytics queries
CREATE INDEX IF NOT EXISTS idx_validation_runs_schema_id ON validation_runs(schema_id);
CREATE INDEX IF NOT EXISTS idx_validation_runs_validated_at ON validation_runs(validated_at);
CREATE INDEX IF NOT EXISTS idx_validation_runs_environment ON validation_runs(environment);
CREATE INDEX IF NOT EXISTS idx_validation_runs_valid ON validation_runs(valid);
CREATE INDEX IF NOT EXISTS idx_validation_runs_schema_method ON validation_runs(schema_id, request_method);

-- Schema comparisons table: stores breaking change analysis results
CREATE TABLE IF NOT EXISTS schema_comparisons (
    id              TEXT PRIMARY KEY,           -- UUID
    schema_id       TEXT NOT NULL,
    old_version     TEXT NOT NULL,
    new_version     TEXT NOT NULL,
    compatible      INTEGER NOT NULL,           -- 1 = compatible, 0 = has breaking changes
    breaking_changes TEXT NOT NULL,             -- JSON array of BreakingChange objects
    compared_at     TEXT NOT NULL,              -- ISO8601 timestamp

    UNIQUE(schema_id, old_version, new_version)
);

CREATE INDEX IF NOT EXISTS idx_schema_comparisons_schema_id ON schema_comparisons(schema_id);
CREATE INDEX IF NOT EXISTS idx_schema_comparisons_compared_at ON schema_comparisons(compared_at);

-- Migration tracking table
CREATE TABLE IF NOT EXISTS migrations (
    version     INTEGER PRIMARY KEY,
    applied_at  TEXT NOT NULL
);

-- Record this migration
INSERT OR IGNORE INTO migrations (version, applied_at) VALUES (1, datetime('now'));
