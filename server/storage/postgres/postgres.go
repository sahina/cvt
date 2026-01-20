// Package postgres provides PostgreSQL storage implementation for CVT.
package postgres

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sahina/cvt/server/pb"
	"github.com/sahina/cvt/server/storage"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func init() {
	storage.Register(storage.StoreTypePostgres, func(ctx context.Context, cfg storage.Config) (storage.Store, error) {
		return New(ctx, cfg)
	})
}

// PostgresStore implements storage.Store using PostgreSQL.
type PostgresStore struct {
	pool   *pgxpool.Pool
	config storage.Config
}

// New creates a new PostgreSQL store.
func New(ctx context.Context, cfg storage.Config) (*PostgresStore, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to parse postgres config: %w", err)
	}

	poolConfig.MaxConns = int32(cfg.MaxConnections)
	poolConfig.MinConns = int32(cfg.MaxIdleConns)
	poolConfig.MaxConnLifetime = cfg.ConnMaxLifetime

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create postgres pool: %w", err)
	}

	return &PostgresStore{pool: pool, config: cfg}, nil
}

// Migrate runs database migrations.
func (s *PostgresStore) Migrate(ctx context.Context) error {
	// Run initial migration
	content, err := migrationsFS.ReadFile("migrations/001_init.sql")
	if err != nil {
		return fmt.Errorf("failed to read migration file 001: %w", err)
	}

	if _, err := s.pool.Exec(ctx, string(content)); err != nil {
		return fmt.Errorf("migration 001 failed: %w", err)
	}

	// Run consumer registry migration
	content, err = migrationsFS.ReadFile("migrations/002_consumers.sql")
	if err != nil {
		return fmt.Errorf("failed to read migration file 002: %w", err)
	}

	if _, err := s.pool.Exec(ctx, string(content)); err != nil {
		return fmt.Errorf("migration 002 failed: %w", err)
	}

	return nil
}

// Close closes the database connection pool.
func (s *PostgresStore) Close() error {
	s.pool.Close()
	return nil
}

// Ping verifies database connectivity.
func (s *PostgresStore) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// SetSchema stores or updates a schema record.
func (s *PostgresStore) SetSchema(ctx context.Context, record *storage.SchemaRecord) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Generate ID if not set
	if record.ID == "" {
		record.ID = uuid.New().String()
	}

	// Mark existing versions as not latest
	_, err = tx.Exec(ctx,
		`UPDATE schemas SET is_latest = FALSE WHERE schema_id = $1`,
		record.SchemaID)
	if err != nil {
		return fmt.Errorf("failed to update latest flag: %w", err)
	}

	// Insert or update the schema
	_, err = tx.Exec(ctx, `
		INSERT INTO schemas (
			id, schema_id, version, content, content_hash,
			openapi_version, endpoint_count, is_latest,
			registered_at, updated_at, owner, team,
			contact_email, read_only, environment
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT(schema_id, version) DO UPDATE SET
			content = EXCLUDED.content,
			content_hash = EXCLUDED.content_hash,
			openapi_version = EXCLUDED.openapi_version,
			endpoint_count = EXCLUDED.endpoint_count,
			is_latest = EXCLUDED.is_latest,
			updated_at = EXCLUDED.updated_at,
			owner = EXCLUDED.owner,
			team = EXCLUDED.team,
			contact_email = EXCLUDED.contact_email,
			read_only = EXCLUDED.read_only,
			environment = EXCLUDED.environment
	`,
		record.ID,
		record.SchemaID,
		record.Version,
		record.Content,
		record.ContentHash,
		record.OpenAPIVersion,
		record.EndpointCount,
		record.IsLatest,
		record.RegisteredAt,
		record.UpdatedAt,
		ownershipOwner(record.Ownership),
		ownershipTeam(record.Ownership),
		ownershipEmail(record.Ownership),
		ownershipReadOnly(record.Ownership),
		record.Environment,
	)
	if err != nil {
		return fmt.Errorf("failed to insert schema: %w", err)
	}

	return tx.Commit(ctx)
}

// GetSchema retrieves the latest version of a schema.
func (s *PostgresStore) GetSchema(ctx context.Context, schemaID string) (*storage.SchemaRecord, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, schema_id, version, content, content_hash,
			   openapi_version, endpoint_count, is_latest,
			   registered_at, updated_at, owner, team,
			   contact_email, read_only, environment
		FROM schemas
		WHERE schema_id = $1 AND is_latest = TRUE
	`, schemaID)

	return scanSchemaRecord(row)
}

// GetSchemaVersion retrieves a specific version of a schema.
func (s *PostgresStore) GetSchemaVersion(ctx context.Context, schemaID, version string) (*storage.SchemaRecord, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, schema_id, version, content, content_hash,
			   openapi_version, endpoint_count, is_latest,
			   registered_at, updated_at, owner, team,
			   contact_email, read_only, environment
		FROM schemas
		WHERE schema_id = $1 AND version = $2
	`, schemaID, version)

	return scanSchemaRecord(row)
}

// DeleteSchema removes all versions of a schema.
func (s *PostgresStore) DeleteSchema(ctx context.Context, schemaID string) error {
	result, err := s.pool.Exec(ctx, `DELETE FROM schemas WHERE schema_id = $1`, schemaID)
	if err != nil {
		return fmt.Errorf("failed to delete schema: %w", err)
	}

	if result.RowsAffected() == 0 {
		return &storage.ErrNotFound{Resource: "schema", ID: schemaID}
	}

	return nil
}

// DeleteSchemaVersion removes a specific version of a schema.
func (s *PostgresStore) DeleteSchemaVersion(ctx context.Context, schemaID, version string) error {
	result, err := s.pool.Exec(ctx,
		`DELETE FROM schemas WHERE schema_id = $1 AND version = $2`,
		schemaID, version)
	if err != nil {
		return fmt.Errorf("failed to delete schema version: %w", err)
	}

	if result.RowsAffected() == 0 {
		return &storage.ErrNotFound{Resource: "schema version", ID: schemaID + "@" + version}
	}

	return nil
}

// ListSchemaIDs returns all unique schema IDs.
func (s *PostgresStore) ListSchemaIDs(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT DISTINCT schema_id FROM schemas ORDER BY schema_id`)
	if err != nil {
		return nil, fmt.Errorf("failed to list schema IDs: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan schema ID: %w", err)
		}
		ids = append(ids, id)
	}

	return ids, rows.Err()
}

// ListVersions returns all versions of a schema.
func (s *PostgresStore) ListVersions(ctx context.Context, schemaID string) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT version FROM schemas WHERE schema_id = $1 ORDER BY registered_at DESC`,
		schemaID)
	if err != nil {
		return nil, fmt.Errorf("failed to list versions: %w", err)
	}
	defer rows.Close()

	var versions []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("failed to scan version: %w", err)
		}
		versions = append(versions, v)
	}

	return versions, rows.Err()
}

// ListSchemas returns schemas matching the filter.
func (s *PostgresStore) ListSchemas(ctx context.Context, filter storage.ListSchemasFilter) ([]*storage.SchemaRecord, string, int32, error) {
	// Build query with filters
	query := `SELECT id, schema_id, version, content, content_hash,
			   openapi_version, endpoint_count, is_latest,
			   registered_at, updated_at, owner, team,
			   contact_email, read_only, environment
		FROM schemas WHERE is_latest = TRUE`

	args := []interface{}{}
	argNum := 1

	if filter.Owner != "" {
		query += fmt.Sprintf(" AND owner = $%d", argNum)
		args = append(args, filter.Owner)
		argNum++
	}
	if filter.Team != "" {
		query += fmt.Sprintf(" AND team = $%d", argNum)
		args = append(args, filter.Team)
		argNum++
	}
	if filter.Environment != "" {
		query += fmt.Sprintf(" AND environment = $%d", argNum)
		args = append(args, filter.Environment)
	}

	// Get total count
	var totalCount int32
	countQuery := "SELECT COUNT(*) FROM schemas WHERE is_latest = TRUE"
	if err := s.pool.QueryRow(ctx, countQuery).Scan(&totalCount); err != nil {
		return nil, "", 0, fmt.Errorf("failed to count schemas: %w", err)
	}

	// Add ordering and pagination
	query += " ORDER BY schema_id"

	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 100
	}
	query += fmt.Sprintf(" LIMIT %d", pageSize+1)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to list schemas: %w", err)
	}
	defer rows.Close()

	var schemas []*storage.SchemaRecord
	for rows.Next() {
		record, err := scanSchemaRecordFromRows(rows)
		if err != nil {
			return nil, "", 0, err
		}
		schemas = append(schemas, record)
	}

	// Check if there are more results
	var nextPageToken string
	if len(schemas) > int(pageSize) {
		schemas = schemas[:pageSize]
		nextPageToken = fmt.Sprintf("%d", len(schemas))
	}

	return schemas, nextPageToken, totalCount, rows.Err()
}

// GetPreviousVersion returns the version before the current one.
func (s *PostgresStore) GetPreviousVersion(ctx context.Context, schemaID, currentVersion string) (string, error) {
	var previousVersion string
	err := s.pool.QueryRow(ctx, `
		SELECT version FROM schemas
		WHERE schema_id = $1 AND registered_at < (
			SELECT registered_at FROM schemas WHERE schema_id = $1 AND version = $2
		)
		ORDER BY registered_at DESC
		LIMIT 1
	`, schemaID, currentVersion).Scan(&previousVersion)

	if err == pgx.ErrNoRows {
		return "", &storage.ErrNotFound{Resource: "previous version", ID: schemaID}
	}
	if err != nil {
		return "", fmt.Errorf("failed to get previous version: %w", err)
	}

	return previousVersion, nil
}

// RecordValidation stores a validation run.
func (s *PostgresStore) RecordValidation(ctx context.Context, record *storage.ValidationRecord) error {
	if record.ID == "" {
		record.ID = uuid.New().String()
	}

	headersJSON, _ := json.Marshal(record.RequestHeaders)
	respHeadersJSON, _ := json.Marshal(record.ResponseHeaders)
	errorsJSON, _ := json.Marshal(record.Errors)

	_, err := s.pool.Exec(ctx, `
		INSERT INTO validation_runs (
			id, schema_id, schema_version, schema_hash,
			request_method, request_path, request_headers, request_body,
			response_status, response_headers, response_body,
			valid, errors, duration_ms, validated_at,
			environment, client_id, client_ip
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
	`,
		record.ID,
		record.SchemaID,
		record.SchemaVersion,
		record.SchemaHash,
		record.RequestMethod,
		record.RequestPath,
		headersJSON,
		record.RequestBody,
		record.ResponseStatus,
		respHeadersJSON,
		record.ResponseBody,
		record.Valid,
		errorsJSON,
		record.DurationMs,
		record.ValidatedAt,
		record.Environment,
		record.ClientID,
		record.ClientIP,
	)

	if err != nil {
		return fmt.Errorf("failed to record validation: %w", err)
	}

	return nil
}

// ListValidations returns validation runs matching the filter.
func (s *PostgresStore) ListValidations(ctx context.Context, filter storage.ListValidationsFilter) ([]*storage.ValidationRecord, string, error) {
	query := `SELECT id, schema_id, schema_version, schema_hash,
			   request_method, request_path, request_headers, request_body,
			   response_status, response_headers, response_body,
			   valid, errors, duration_ms, validated_at,
			   environment, client_id, client_ip
		FROM validation_runs WHERE 1=1`

	args := []interface{}{}
	argNum := 1

	if filter.SchemaID != "" {
		query += fmt.Sprintf(" AND schema_id = $%d", argNum)
		args = append(args, filter.SchemaID)
		argNum++
	}
	if filter.Method != "" {
		query += fmt.Sprintf(" AND request_method = $%d", argNum)
		args = append(args, filter.Method)
		argNum++
	}
	if filter.Environment != "" {
		query += fmt.Sprintf(" AND environment = $%d", argNum)
		args = append(args, filter.Environment)
		argNum++
	}
	if filter.Valid != nil {
		query += fmt.Sprintf(" AND valid = $%d", argNum)
		args = append(args, *filter.Valid)
		argNum++
	}
	if !filter.StartTime.IsZero() {
		query += fmt.Sprintf(" AND validated_at >= $%d", argNum)
		args = append(args, filter.StartTime)
		argNum++
	}
	if !filter.EndTime.IsZero() {
		query += fmt.Sprintf(" AND validated_at <= $%d", argNum)
		args = append(args, filter.EndTime)
	}

	query += " ORDER BY validated_at DESC"

	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 100
	}
	query += fmt.Sprintf(" LIMIT %d", pageSize)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("failed to list validations: %w", err)
	}
	defer rows.Close()

	var records []*storage.ValidationRecord
	for rows.Next() {
		record, err := scanValidationRecord(rows)
		if err != nil {
			return nil, "", err
		}
		records = append(records, record)
	}

	return records, "", rows.Err()
}

// GetValidationAnalytics returns aggregated validation statistics.
func (s *PostgresStore) GetValidationAnalytics(ctx context.Context, filter storage.ListValidationsFilter) (*storage.ValidationAnalytics, error) {
	analytics := &storage.ValidationAnalytics{
		ByMethod: make(map[string]int64),
		BySchema: make(map[string]int64),
	}

	// Base where clause
	whereClause := "WHERE 1=1"
	args := []interface{}{}
	argNum := 1

	if filter.SchemaID != "" {
		whereClause += fmt.Sprintf(" AND schema_id = $%d", argNum)
		args = append(args, filter.SchemaID)
		argNum++
	}
	if !filter.StartTime.IsZero() {
		whereClause += fmt.Sprintf(" AND validated_at >= $%d", argNum)
		args = append(args, filter.StartTime)
		argNum++
	}
	if !filter.EndTime.IsZero() {
		whereClause += fmt.Sprintf(" AND validated_at <= $%d", argNum)
		args = append(args, filter.EndTime)
	}

	// Get total counts
	err := s.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT
			COUNT(*) as total,
			SUM(CASE WHEN valid = TRUE THEN 1 ELSE 0 END) as pass_count,
			SUM(CASE WHEN valid = FALSE THEN 1 ELSE 0 END) as fail_count
		FROM validation_runs %s
	`, whereClause), args...).Scan(&analytics.TotalValidations, &analytics.PassCount, &analytics.FailCount)

	if err != nil {
		return nil, fmt.Errorf("failed to get analytics: %w", err)
	}

	if analytics.TotalValidations > 0 {
		analytics.PassRate = float64(analytics.PassCount) / float64(analytics.TotalValidations) * 100
	}

	// Get counts by method
	methodRows, err := s.pool.Query(ctx, fmt.Sprintf(`
		SELECT request_method, COUNT(*) as count
		FROM validation_runs %s
		GROUP BY request_method
	`, whereClause), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get method analytics: %w", err)
	}
	defer methodRows.Close()

	for methodRows.Next() {
		var method string
		var count int64
		if err := methodRows.Scan(&method, &count); err != nil {
			return nil, err
		}
		analytics.ByMethod[method] = count
	}

	// Get counts by schema
	schemaRows, err := s.pool.Query(ctx, fmt.Sprintf(`
		SELECT schema_id, COUNT(*) as count
		FROM validation_runs %s
		GROUP BY schema_id
		ORDER BY count DESC
		LIMIT 10
	`, whereClause), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get schema analytics: %w", err)
	}
	defer schemaRows.Close()

	for schemaRows.Next() {
		var schemaID string
		var count int64
		if err := schemaRows.Scan(&schemaID, &count); err != nil {
			return nil, err
		}
		analytics.BySchema[schemaID] = count
	}

	return analytics, nil
}

// RecordComparison stores a schema comparison.
func (s *PostgresStore) RecordComparison(ctx context.Context, record *storage.ComparisonRecord) error {
	if record.ID == "" {
		record.ID = uuid.New().String()
	}

	changesJSON, _ := json.Marshal(record.BreakingChanges)

	_, err := s.pool.Exec(ctx, `
		INSERT INTO schema_comparisons (
			id, schema_id, old_version, new_version,
			compatible, breaking_changes, compared_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT(schema_id, old_version, new_version) DO UPDATE SET
			compatible = EXCLUDED.compatible,
			breaking_changes = EXCLUDED.breaking_changes,
			compared_at = EXCLUDED.compared_at
	`,
		record.ID,
		record.SchemaID,
		record.OldVersion,
		record.NewVersion,
		record.Compatible,
		changesJSON,
		record.ComparedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to record comparison: %w", err)
	}

	return nil
}

// GetComparison retrieves a stored comparison.
func (s *PostgresStore) GetComparison(ctx context.Context, schemaID, oldVersion, newVersion string) (*storage.ComparisonRecord, error) {
	var record storage.ComparisonRecord
	var changesJSON []byte

	err := s.pool.QueryRow(ctx, `
		SELECT id, schema_id, old_version, new_version,
			   compatible, breaking_changes, compared_at
		FROM schema_comparisons
		WHERE schema_id = $1 AND old_version = $2 AND new_version = $3
	`, schemaID, oldVersion, newVersion).Scan(
		&record.ID,
		&record.SchemaID,
		&record.OldVersion,
		&record.NewVersion,
		&record.Compatible,
		&changesJSON,
		&record.ComparedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, &storage.ErrNotFound{Resource: "comparison", ID: schemaID}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get comparison: %w", err)
	}

	_ = json.Unmarshal(changesJSON, &record.BreakingChanges)

	return &record, nil
}

// Helper functions

func scanSchemaRecord(row pgx.Row) (*storage.SchemaRecord, error) {
	var record storage.SchemaRecord
	var owner, team, email *string
	var readOnly bool

	err := row.Scan(
		&record.ID,
		&record.SchemaID,
		&record.Version,
		&record.Content,
		&record.ContentHash,
		&record.OpenAPIVersion,
		&record.EndpointCount,
		&record.IsLatest,
		&record.RegisteredAt,
		&record.UpdatedAt,
		&owner,
		&team,
		&email,
		&readOnly,
		&record.Environment,
	)

	if err == pgx.ErrNoRows {
		return nil, &storage.ErrNotFound{Resource: "schema", ID: "unknown"}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan schema: %w", err)
	}

	if owner != nil || team != nil || email != nil {
		record.Ownership = &pb.SchemaOwnership{
			ReadOnly: readOnly,
		}
		if owner != nil {
			record.Ownership.Owner = *owner
		}
		if team != nil {
			record.Ownership.Team = *team
		}
		if email != nil {
			record.Ownership.ContactEmail = *email
		}
	}

	return &record, nil
}

func scanSchemaRecordFromRows(rows pgx.Rows) (*storage.SchemaRecord, error) {
	var record storage.SchemaRecord
	var owner, team, email *string
	var readOnly bool

	err := rows.Scan(
		&record.ID,
		&record.SchemaID,
		&record.Version,
		&record.Content,
		&record.ContentHash,
		&record.OpenAPIVersion,
		&record.EndpointCount,
		&record.IsLatest,
		&record.RegisteredAt,
		&record.UpdatedAt,
		&owner,
		&team,
		&email,
		&readOnly,
		&record.Environment,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to scan schema: %w", err)
	}

	if owner != nil || team != nil || email != nil {
		record.Ownership = &pb.SchemaOwnership{
			ReadOnly: readOnly,
		}
		if owner != nil {
			record.Ownership.Owner = *owner
		}
		if team != nil {
			record.Ownership.Team = *team
		}
		if email != nil {
			record.Ownership.ContactEmail = *email
		}
	}

	return &record, nil
}

func scanValidationRecord(rows pgx.Rows) (*storage.ValidationRecord, error) {
	var record storage.ValidationRecord
	var headersJSON, respHeadersJSON, errorsJSON []byte
	var clientID, clientIP *string

	err := rows.Scan(
		&record.ID,
		&record.SchemaID,
		&record.SchemaVersion,
		&record.SchemaHash,
		&record.RequestMethod,
		&record.RequestPath,
		&headersJSON,
		&record.RequestBody,
		&record.ResponseStatus,
		&respHeadersJSON,
		&record.ResponseBody,
		&record.Valid,
		&errorsJSON,
		&record.DurationMs,
		&record.ValidatedAt,
		&record.Environment,
		&clientID,
		&clientIP,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to scan validation: %w", err)
	}

	if clientID != nil {
		record.ClientID = *clientID
	}
	if clientIP != nil {
		record.ClientIP = *clientIP
	}

	_ = json.Unmarshal(headersJSON, &record.RequestHeaders)
	_ = json.Unmarshal(respHeadersJSON, &record.ResponseHeaders)
	_ = json.Unmarshal(errorsJSON, &record.Errors)

	return &record, nil
}

func ownershipOwner(o *pb.SchemaOwnership) *string {
	if o == nil || o.Owner == "" {
		return nil
	}
	return &o.Owner
}

func ownershipTeam(o *pb.SchemaOwnership) *string {
	if o == nil || o.Team == "" {
		return nil
	}
	return &o.Team
}

func ownershipEmail(o *pb.SchemaOwnership) *string {
	if o == nil || o.ContactEmail == "" {
		return nil
	}
	return &o.ContactEmail
}

func ownershipReadOnly(o *pb.SchemaOwnership) bool {
	if o == nil {
		return false
	}
	return o.ReadOnly
}

// RegisterConsumer stores or updates a consumer registration.
func (s *PostgresStore) RegisterConsumer(ctx context.Context, record *storage.ConsumerRecord) error {
	if record.ID == "" {
		record.ID = uuid.New().String()
	}

	endpointsJSON, err := json.Marshal(record.UsedEndpoints)
	if err != nil {
		return fmt.Errorf("failed to marshal endpoints: %w", err)
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO consumers (
			id, consumer_id, consumer_version, schema_id, schema_version,
			environment, registered_at, last_validated_at, used_endpoints
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT(consumer_id, environment) DO UPDATE SET
			consumer_version = EXCLUDED.consumer_version,
			schema_id = EXCLUDED.schema_id,
			schema_version = EXCLUDED.schema_version,
			last_validated_at = EXCLUDED.last_validated_at,
			used_endpoints = EXCLUDED.used_endpoints
	`,
		record.ID,
		record.ConsumerID,
		record.ConsumerVersion,
		record.SchemaID,
		record.SchemaVersion,
		record.Environment,
		record.RegisteredAt,
		record.LastValidatedAt,
		endpointsJSON,
	)

	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	return nil
}

// GetConsumer retrieves a consumer registration.
func (s *PostgresStore) GetConsumer(ctx context.Context, consumerID, schemaID, environment string) (*storage.ConsumerRecord, error) {
	var record storage.ConsumerRecord
	var endpointsJSON []byte

	err := s.pool.QueryRow(ctx, `
		SELECT id, consumer_id, consumer_version, schema_id, schema_version,
			   environment, registered_at, last_validated_at, used_endpoints
		FROM consumers
		WHERE consumer_id = $1 AND schema_id = $2 AND environment = $3
	`, consumerID, schemaID, environment).Scan(
		&record.ID,
		&record.ConsumerID,
		&record.ConsumerVersion,
		&record.SchemaID,
		&record.SchemaVersion,
		&record.Environment,
		&record.RegisteredAt,
		&record.LastValidatedAt,
		&endpointsJSON,
	)

	if err == pgx.ErrNoRows {
		return nil, &storage.ErrNotFound{Resource: "consumer", ID: consumerID + "/" + schemaID + "/" + environment}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get consumer: %w", err)
	}

	if len(endpointsJSON) > 0 {
		_ = json.Unmarshal(endpointsJSON, &record.UsedEndpoints)
	}

	return &record, nil
}

// ListConsumers returns consumers matching the filter.
func (s *PostgresStore) ListConsumers(ctx context.Context, filter storage.ListConsumersFilter) ([]*storage.ConsumerRecord, error) {
	query := `SELECT id, consumer_id, consumer_version, schema_id, schema_version,
			   environment, registered_at, last_validated_at, used_endpoints
		FROM consumers WHERE TRUE`

	args := []interface{}{}
	argIdx := 1

	if filter.SchemaID != "" {
		query += fmt.Sprintf(" AND schema_id = $%d", argIdx)
		args = append(args, filter.SchemaID)
		argIdx++
	}
	if filter.Environment != "" {
		query += fmt.Sprintf(" AND environment = $%d", argIdx)
		args = append(args, filter.Environment)
		argIdx++
	}
	if filter.ConsumerID != "" {
		query += fmt.Sprintf(" AND consumer_id = $%d", argIdx)
		args = append(args, filter.ConsumerID)
	}

	query += " ORDER BY consumer_id"

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list consumers: %w", err)
	}
	defer rows.Close()

	var records []*storage.ConsumerRecord
	for rows.Next() {
		var record storage.ConsumerRecord
		var endpointsJSON []byte

		err := rows.Scan(
			&record.ID,
			&record.ConsumerID,
			&record.ConsumerVersion,
			&record.SchemaID,
			&record.SchemaVersion,
			&record.Environment,
			&record.RegisteredAt,
			&record.LastValidatedAt,
			&endpointsJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan consumer: %w", err)
		}

		if len(endpointsJSON) > 0 {
			_ = json.Unmarshal(endpointsJSON, &record.UsedEndpoints)
		}

		records = append(records, &record)
	}

	return records, rows.Err()
}

// DeregisterConsumer removes a consumer registration.
func (s *PostgresStore) DeregisterConsumer(ctx context.Context, consumerID, schemaID, environment string) error {
	result, err := s.pool.Exec(ctx,
		`DELETE FROM consumers WHERE consumer_id = $1 AND schema_id = $2 AND environment = $3`,
		consumerID, schemaID, environment)
	if err != nil {
		return fmt.Errorf("failed to deregister consumer: %w", err)
	}

	if result.RowsAffected() == 0 {
		return &storage.ErrNotFound{Resource: "consumer", ID: consumerID + "/" + schemaID + "/" + environment}
	}

	return nil
}

// UpdateConsumerValidation updates the last validated timestamp for a consumer.
func (s *PostgresStore) UpdateConsumerValidation(ctx context.Context, consumerID, schemaID, environment string, validatedAt time.Time) error {
	result, err := s.pool.Exec(ctx,
		`UPDATE consumers SET last_validated_at = $1 WHERE consumer_id = $2 AND schema_id = $3 AND environment = $4`,
		validatedAt, consumerID, schemaID, environment)
	if err != nil {
		return fmt.Errorf("failed to update consumer validation: %w", err)
	}

	if result.RowsAffected() == 0 {
		return &storage.ErrNotFound{Resource: "consumer", ID: consumerID + "/" + schemaID + "/" + environment}
	}

	return nil
}
