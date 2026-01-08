// Package sqlite provides SQLite storage implementation for CVT.
package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cvt/cvt/server/pb"
	"github.com/cvt/cvt/server/storage"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func init() {
	storage.Register(storage.StoreTypeSQLite, func(ctx context.Context, cfg storage.Config) (storage.Store, error) {
		return New(cfg)
	})
}

// SQLiteStore implements storage.Store using SQLite.
type SQLiteStore struct {
	db     *sql.DB
	config storage.Config
}

// New creates a new SQLite store.
func New(cfg storage.Config) (*SQLiteStore, error) {
	dsn := cfg.DSN
	if dsn == "" {
		dsn = "cvt.db"
	}

	// Add WAL mode and other optimizations
	dsn = fmt.Sprintf("%s?_journal=WAL&_synchronous=NORMAL&_busy_timeout=5000", dsn)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(1) // SQLite only supports single writer
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	return &SQLiteStore{db: db, config: cfg}, nil
}

// Migrate runs database migrations.
func (s *SQLiteStore) Migrate(ctx context.Context) error {
	// Run initial migration
	content, err := migrationsFS.ReadFile("migrations/001_init.sql")
	if err != nil {
		return fmt.Errorf("failed to read migration file 001: %w", err)
	}

	if _, err := s.db.ExecContext(ctx, string(content)); err != nil {
		return fmt.Errorf("migration 001 failed: %w", err)
	}

	// Run consumer registry migration
	content, err = migrationsFS.ReadFile("migrations/002_consumers.sql")
	if err != nil {
		return fmt.Errorf("failed to read migration file 002: %w", err)
	}

	if _, err := s.db.ExecContext(ctx, string(content)); err != nil {
		return fmt.Errorf("migration 002 failed: %w", err)
	}

	return nil
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// Ping verifies database connectivity.
func (s *SQLiteStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// SetSchema stores or updates a schema record.
func (s *SQLiteStore) SetSchema(ctx context.Context, record *storage.SchemaRecord) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Generate ID if not set
	if record.ID == "" {
		record.ID = uuid.New().String()
	}

	// Mark existing versions as not latest
	_, err = tx.ExecContext(ctx,
		`UPDATE schemas SET is_latest = 0 WHERE schema_id = ?`,
		record.SchemaID)
	if err != nil {
		return fmt.Errorf("failed to update latest flag: %w", err)
	}

	// Insert or update the schema
	_, err = tx.ExecContext(ctx, `
		INSERT INTO schemas (
			id, schema_id, version, content, content_hash,
			openapi_version, endpoint_count, is_latest,
			registered_at, updated_at, owner, team,
			contact_email, read_only, environment
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(schema_id, version) DO UPDATE SET
			content = excluded.content,
			content_hash = excluded.content_hash,
			openapi_version = excluded.openapi_version,
			endpoint_count = excluded.endpoint_count,
			is_latest = excluded.is_latest,
			updated_at = excluded.updated_at,
			owner = excluded.owner,
			team = excluded.team,
			contact_email = excluded.contact_email,
			read_only = excluded.read_only,
			environment = excluded.environment
	`,
		record.ID,
		record.SchemaID,
		record.Version,
		record.Content,
		record.ContentHash,
		record.OpenAPIVersion,
		record.EndpointCount,
		boolToInt(record.IsLatest),
		record.RegisteredAt.Format(time.RFC3339),
		record.UpdatedAt.Format(time.RFC3339),
		ownershipOwner(record.Ownership),
		ownershipTeam(record.Ownership),
		ownershipEmail(record.Ownership),
		boolToInt(ownershipReadOnly(record.Ownership)),
		record.Environment,
	)
	if err != nil {
		return fmt.Errorf("failed to insert schema: %w", err)
	}

	return tx.Commit()
}

// GetSchema retrieves the latest version of a schema.
func (s *SQLiteStore) GetSchema(ctx context.Context, schemaID string) (*storage.SchemaRecord, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, schema_id, version, content, content_hash,
			   openapi_version, endpoint_count, is_latest,
			   registered_at, updated_at, owner, team,
			   contact_email, read_only, environment
		FROM schemas
		WHERE schema_id = ? AND is_latest = 1
	`, schemaID)

	return scanSchemaRecord(row)
}

// GetSchemaVersion retrieves a specific version of a schema.
func (s *SQLiteStore) GetSchemaVersion(ctx context.Context, schemaID, version string) (*storage.SchemaRecord, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, schema_id, version, content, content_hash,
			   openapi_version, endpoint_count, is_latest,
			   registered_at, updated_at, owner, team,
			   contact_email, read_only, environment
		FROM schemas
		WHERE schema_id = ? AND version = ?
	`, schemaID, version)

	return scanSchemaRecord(row)
}

// DeleteSchema removes all versions of a schema.
func (s *SQLiteStore) DeleteSchema(ctx context.Context, schemaID string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM schemas WHERE schema_id = ?`, schemaID)
	if err != nil {
		return fmt.Errorf("failed to delete schema: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return &storage.ErrNotFound{Resource: "schema", ID: schemaID}
	}

	return nil
}

// DeleteSchemaVersion removes a specific version of a schema.
func (s *SQLiteStore) DeleteSchemaVersion(ctx context.Context, schemaID, version string) error {
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM schemas WHERE schema_id = ? AND version = ?`,
		schemaID, version)
	if err != nil {
		return fmt.Errorf("failed to delete schema version: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return &storage.ErrNotFound{Resource: "schema version", ID: schemaID + "@" + version}
	}

	return nil
}

// ListSchemaIDs returns all unique schema IDs.
func (s *SQLiteStore) ListSchemaIDs(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT schema_id FROM schemas ORDER BY schema_id`)
	if err != nil {
		return nil, fmt.Errorf("failed to list schema IDs: %w", err)
	}
	defer func() { _ = rows.Close() }()

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
func (s *SQLiteStore) ListVersions(ctx context.Context, schemaID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT version FROM schemas WHERE schema_id = ? ORDER BY registered_at DESC`,
		schemaID)
	if err != nil {
		return nil, fmt.Errorf("failed to list versions: %w", err)
	}
	defer func() { _ = rows.Close() }()

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
func (s *SQLiteStore) ListSchemas(ctx context.Context, filter storage.ListSchemasFilter) ([]*storage.SchemaRecord, string, int32, error) {
	// Build query with filters
	query := `SELECT id, schema_id, version, content, content_hash,
			   openapi_version, endpoint_count, is_latest,
			   registered_at, updated_at, owner, team,
			   contact_email, read_only, environment
		FROM schemas WHERE is_latest = 1`

	args := []interface{}{}

	if filter.Owner != "" {
		query += " AND owner = ?"
		args = append(args, filter.Owner)
	}
	if filter.Team != "" {
		query += " AND team = ?"
		args = append(args, filter.Team)
	}
	if filter.Environment != "" {
		query += " AND environment = ?"
		args = append(args, filter.Environment)
	}

	// Get total count
	countQuery := "SELECT COUNT(*) FROM schemas WHERE is_latest = 1"
	var totalCount int32
	if err := s.db.QueryRowContext(ctx, countQuery).Scan(&totalCount); err != nil {
		return nil, "", 0, fmt.Errorf("failed to count schemas: %w", err)
	}

	// Add ordering and pagination
	query += " ORDER BY schema_id"

	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 100
	}
	query += fmt.Sprintf(" LIMIT %d", pageSize+1) // +1 to detect if there are more

	if filter.PageToken != "" {
		query += " OFFSET ?"
		args = append(args, filter.PageToken)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to list schemas: %w", err)
	}
	defer func() { _ = rows.Close() }()

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
func (s *SQLiteStore) GetPreviousVersion(ctx context.Context, schemaID, currentVersion string) (string, error) {
	var previousVersion string
	err := s.db.QueryRowContext(ctx, `
		SELECT version FROM schemas
		WHERE schema_id = ? AND registered_at < (
			SELECT registered_at FROM schemas WHERE schema_id = ? AND version = ?
		)
		ORDER BY registered_at DESC
		LIMIT 1
	`, schemaID, schemaID, currentVersion).Scan(&previousVersion)

	if err == sql.ErrNoRows {
		return "", &storage.ErrNotFound{Resource: "previous version", ID: schemaID}
	}
	if err != nil {
		return "", fmt.Errorf("failed to get previous version: %w", err)
	}

	return previousVersion, nil
}

// RecordValidation stores a validation run.
func (s *SQLiteStore) RecordValidation(ctx context.Context, record *storage.ValidationRecord) error {
	if record.ID == "" {
		record.ID = uuid.New().String()
	}

	headersJSON, _ := json.Marshal(record.RequestHeaders)
	respHeadersJSON, _ := json.Marshal(record.ResponseHeaders)
	errorsJSON, _ := json.Marshal(record.Errors)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO validation_runs (
			id, schema_id, schema_version, schema_hash,
			request_method, request_path, request_headers, request_body,
			response_status, response_headers, response_body,
			valid, errors, duration_ms, validated_at,
			environment, client_id, client_ip
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		record.ID,
		record.SchemaID,
		record.SchemaVersion,
		record.SchemaHash,
		record.RequestMethod,
		record.RequestPath,
		string(headersJSON),
		record.RequestBody,
		record.ResponseStatus,
		string(respHeadersJSON),
		record.ResponseBody,
		boolToInt(record.Valid),
		string(errorsJSON),
		record.DurationMs,
		record.ValidatedAt.Format(time.RFC3339),
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
func (s *SQLiteStore) ListValidations(ctx context.Context, filter storage.ListValidationsFilter) ([]*storage.ValidationRecord, string, error) {
	query := `SELECT id, schema_id, schema_version, schema_hash,
			   request_method, request_path, request_headers, request_body,
			   response_status, response_headers, response_body,
			   valid, errors, duration_ms, validated_at,
			   environment, client_id, client_ip
		FROM validation_runs WHERE 1=1`

	args := []interface{}{}

	if filter.SchemaID != "" {
		query += " AND schema_id = ?"
		args = append(args, filter.SchemaID)
	}
	if filter.Method != "" {
		query += " AND request_method = ?"
		args = append(args, filter.Method)
	}
	if filter.Environment != "" {
		query += " AND environment = ?"
		args = append(args, filter.Environment)
	}
	if filter.Valid != nil {
		query += " AND valid = ?"
		args = append(args, boolToInt(*filter.Valid))
	}
	if !filter.StartTime.IsZero() {
		query += " AND validated_at >= ?"
		args = append(args, filter.StartTime.Format(time.RFC3339))
	}
	if !filter.EndTime.IsZero() {
		query += " AND validated_at <= ?"
		args = append(args, filter.EndTime.Format(time.RFC3339))
	}

	query += " ORDER BY validated_at DESC"

	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 100
	}
	query += fmt.Sprintf(" LIMIT %d", pageSize)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("failed to list validations: %w", err)
	}
	defer func() { _ = rows.Close() }()

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
func (s *SQLiteStore) GetValidationAnalytics(ctx context.Context, filter storage.ListValidationsFilter) (*storage.ValidationAnalytics, error) {
	analytics := &storage.ValidationAnalytics{
		ByMethod: make(map[string]int64),
		BySchema: make(map[string]int64),
	}

	// Base where clause
	whereClause := "WHERE 1=1"
	args := []interface{}{}

	if filter.SchemaID != "" {
		whereClause += " AND schema_id = ?"
		args = append(args, filter.SchemaID)
	}
	if !filter.StartTime.IsZero() {
		whereClause += " AND validated_at >= ?"
		args = append(args, filter.StartTime.Format(time.RFC3339))
	}
	if !filter.EndTime.IsZero() {
		whereClause += " AND validated_at <= ?"
		args = append(args, filter.EndTime.Format(time.RFC3339))
	}

	// Get total counts
	err := s.db.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT
			COUNT(*) as total,
			SUM(CASE WHEN valid = 1 THEN 1 ELSE 0 END) as pass_count,
			SUM(CASE WHEN valid = 0 THEN 1 ELSE 0 END) as fail_count
		FROM validation_runs %s
	`, whereClause), args...).Scan(&analytics.TotalValidations, &analytics.PassCount, &analytics.FailCount)

	if err != nil {
		return nil, fmt.Errorf("failed to get analytics: %w", err)
	}

	if analytics.TotalValidations > 0 {
		analytics.PassRate = float64(analytics.PassCount) / float64(analytics.TotalValidations) * 100
	}

	// Get counts by method
	methodRows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT request_method, COUNT(*) as count
		FROM validation_runs %s
		GROUP BY request_method
	`, whereClause), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get method analytics: %w", err)
	}
	defer func() { _ = methodRows.Close() }()

	for methodRows.Next() {
		var method string
		var count int64
		if err := methodRows.Scan(&method, &count); err != nil {
			return nil, err
		}
		analytics.ByMethod[method] = count
	}

	// Get counts by schema
	schemaRows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT schema_id, COUNT(*) as count
		FROM validation_runs %s
		GROUP BY schema_id
		ORDER BY count DESC
		LIMIT 10
	`, whereClause), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get schema analytics: %w", err)
	}
	defer func() { _ = schemaRows.Close() }()

	for schemaRows.Next() {
		var schemaID string
		var count int64
		if err := schemaRows.Scan(&schemaID, &count); err != nil {
			return nil, err
		}
		analytics.BySchema[schemaID] = count
	}

	// Get top errors
	errorRows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT errors, COUNT(*) as count
		FROM validation_runs %s
		WHERE valid = 0 AND errors != '[]' AND errors != ''
		GROUP BY errors
		ORDER BY count DESC
		LIMIT 10
	`, whereClause), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get error analytics: %w", err)
	}
	defer func() { _ = errorRows.Close() }()

	for errorRows.Next() {
		var errorsJSON string
		var count int64
		if err := errorRows.Scan(&errorsJSON, &count); err != nil {
			return nil, err
		}
		var errors []string
		_ = json.Unmarshal([]byte(errorsJSON), &errors)
		for _, e := range errors {
			analytics.TopErrors = append(analytics.TopErrors, storage.ErrorCount{
				Error: e,
				Count: count,
			})
		}
	}

	return analytics, nil
}

// RecordComparison stores a schema comparison.
func (s *SQLiteStore) RecordComparison(ctx context.Context, record *storage.ComparisonRecord) error {
	if record.ID == "" {
		record.ID = uuid.New().String()
	}

	changesJSON, _ := json.Marshal(record.BreakingChanges)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO schema_comparisons (
			id, schema_id, old_version, new_version,
			compatible, breaking_changes, compared_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(schema_id, old_version, new_version) DO UPDATE SET
			compatible = excluded.compatible,
			breaking_changes = excluded.breaking_changes,
			compared_at = excluded.compared_at
	`,
		record.ID,
		record.SchemaID,
		record.OldVersion,
		record.NewVersion,
		boolToInt(record.Compatible),
		string(changesJSON),
		record.ComparedAt.Format(time.RFC3339),
	)

	if err != nil {
		return fmt.Errorf("failed to record comparison: %w", err)
	}

	return nil
}

// GetComparison retrieves a stored comparison.
func (s *SQLiteStore) GetComparison(ctx context.Context, schemaID, oldVersion, newVersion string) (*storage.ComparisonRecord, error) {
	var record storage.ComparisonRecord
	var compatible int
	var changesJSON string
	var comparedAt string

	err := s.db.QueryRowContext(ctx, `
		SELECT id, schema_id, old_version, new_version,
			   compatible, breaking_changes, compared_at
		FROM schema_comparisons
		WHERE schema_id = ? AND old_version = ? AND new_version = ?
	`, schemaID, oldVersion, newVersion).Scan(
		&record.ID,
		&record.SchemaID,
		&record.OldVersion,
		&record.NewVersion,
		&compatible,
		&changesJSON,
		&comparedAt,
	)

	if err == sql.ErrNoRows {
		return nil, &storage.ErrNotFound{Resource: "comparison", ID: schemaID}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get comparison: %w", err)
	}

	record.Compatible = compatible == 1
	_ = json.Unmarshal([]byte(changesJSON), &record.BreakingChanges)
	record.ComparedAt, _ = time.Parse(time.RFC3339, comparedAt)

	return &record, nil
}

// Helper functions

func scanSchemaRecord(row *sql.Row) (*storage.SchemaRecord, error) {
	var record storage.SchemaRecord
	var isLatest, readOnly int
	var registeredAt, updatedAt string
	var owner, team, email sql.NullString

	err := row.Scan(
		&record.ID,
		&record.SchemaID,
		&record.Version,
		&record.Content,
		&record.ContentHash,
		&record.OpenAPIVersion,
		&record.EndpointCount,
		&isLatest,
		&registeredAt,
		&updatedAt,
		&owner,
		&team,
		&email,
		&readOnly,
		&record.Environment,
	)

	if err == sql.ErrNoRows {
		return nil, &storage.ErrNotFound{Resource: "schema", ID: "unknown"}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan schema: %w", err)
	}

	record.IsLatest = isLatest == 1
	record.RegisteredAt, _ = time.Parse(time.RFC3339, registeredAt)
	record.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	if owner.Valid || team.Valid || email.Valid {
		record.Ownership = &pb.SchemaOwnership{
			Owner:        owner.String,
			Team:         team.String,
			ContactEmail: email.String,
			ReadOnly:     readOnly == 1,
		}
	}

	return &record, nil
}

func scanSchemaRecordFromRows(rows *sql.Rows) (*storage.SchemaRecord, error) {
	var record storage.SchemaRecord
	var isLatest, readOnly int
	var registeredAt, updatedAt string
	var owner, team, email sql.NullString

	err := rows.Scan(
		&record.ID,
		&record.SchemaID,
		&record.Version,
		&record.Content,
		&record.ContentHash,
		&record.OpenAPIVersion,
		&record.EndpointCount,
		&isLatest,
		&registeredAt,
		&updatedAt,
		&owner,
		&team,
		&email,
		&readOnly,
		&record.Environment,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to scan schema: %w", err)
	}

	record.IsLatest = isLatest == 1
	record.RegisteredAt, _ = time.Parse(time.RFC3339, registeredAt)
	record.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	if owner.Valid || team.Valid || email.Valid {
		record.Ownership = &pb.SchemaOwnership{
			Owner:        owner.String,
			Team:         team.String,
			ContactEmail: email.String,
			ReadOnly:     readOnly == 1,
		}
	}

	return &record, nil
}

func scanValidationRecord(rows *sql.Rows) (*storage.ValidationRecord, error) {
	var record storage.ValidationRecord
	var valid int
	var validatedAt string
	var headersJSON, respHeadersJSON, errorsJSON sql.NullString
	var clientID, clientIP sql.NullString

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
		&valid,
		&errorsJSON,
		&record.DurationMs,
		&validatedAt,
		&record.Environment,
		&clientID,
		&clientIP,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to scan validation: %w", err)
	}

	record.Valid = valid == 1
	record.ValidatedAt, _ = time.Parse(time.RFC3339, validatedAt)
	record.ClientID = clientID.String
	record.ClientIP = clientIP.String

	if headersJSON.Valid {
		_ = json.Unmarshal([]byte(headersJSON.String), &record.RequestHeaders)
	}
	if respHeadersJSON.Valid {
		_ = json.Unmarshal([]byte(respHeadersJSON.String), &record.ResponseHeaders)
	}
	if errorsJSON.Valid {
		_ = json.Unmarshal([]byte(errorsJSON.String), &record.Errors)
	}

	return &record, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func ownershipOwner(o *pb.SchemaOwnership) string {
	if o == nil {
		return ""
	}
	return o.Owner
}

func ownershipTeam(o *pb.SchemaOwnership) string {
	if o == nil {
		return ""
	}
	return o.Team
}

func ownershipEmail(o *pb.SchemaOwnership) string {
	if o == nil {
		return ""
	}
	return o.ContactEmail
}

func ownershipReadOnly(o *pb.SchemaOwnership) bool {
	if o == nil {
		return false
	}
	return o.ReadOnly
}

// RegisterConsumer stores or updates a consumer registration.
func (s *SQLiteStore) RegisterConsumer(ctx context.Context, record *storage.ConsumerRecord) error {
	if record.ID == "" {
		record.ID = uuid.New().String()
	}

	endpointsJSON, err := json.Marshal(record.UsedEndpoints)
	if err != nil {
		return fmt.Errorf("failed to marshal endpoints: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO consumers (
			id, consumer_id, consumer_version, schema_id, schema_version,
			environment, registered_at, last_validated_at, used_endpoints
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(consumer_id, schema_id, environment) DO UPDATE SET
			consumer_version = excluded.consumer_version,
			schema_version = excluded.schema_version,
			last_validated_at = excluded.last_validated_at,
			used_endpoints = excluded.used_endpoints
	`,
		record.ID,
		record.ConsumerID,
		record.ConsumerVersion,
		record.SchemaID,
		record.SchemaVersion,
		record.Environment,
		record.RegisteredAt.Format(time.RFC3339),
		record.LastValidatedAt.Format(time.RFC3339),
		string(endpointsJSON),
	)

	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	return nil
}

// GetConsumer retrieves a consumer registration.
func (s *SQLiteStore) GetConsumer(ctx context.Context, consumerID, schemaID, environment string) (*storage.ConsumerRecord, error) {
	var record storage.ConsumerRecord
	var registeredAt, lastValidatedAt string
	var endpointsJSON sql.NullString

	err := s.db.QueryRowContext(ctx, `
		SELECT id, consumer_id, consumer_version, schema_id, schema_version,
			   environment, registered_at, last_validated_at, used_endpoints
		FROM consumers
		WHERE consumer_id = ? AND schema_id = ? AND environment = ?
	`, consumerID, schemaID, environment).Scan(
		&record.ID,
		&record.ConsumerID,
		&record.ConsumerVersion,
		&record.SchemaID,
		&record.SchemaVersion,
		&record.Environment,
		&registeredAt,
		&lastValidatedAt,
		&endpointsJSON,
	)

	if err == sql.ErrNoRows {
		return nil, &storage.ErrNotFound{Resource: "consumer", ID: consumerID + "/" + schemaID + "/" + environment}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get consumer: %w", err)
	}

	record.RegisteredAt, _ = time.Parse(time.RFC3339, registeredAt)
	record.LastValidatedAt, _ = time.Parse(time.RFC3339, lastValidatedAt)

	if endpointsJSON.Valid {
		_ = json.Unmarshal([]byte(endpointsJSON.String), &record.UsedEndpoints)
	}

	return &record, nil
}

// ListConsumers returns consumers matching the filter.
func (s *SQLiteStore) ListConsumers(ctx context.Context, filter storage.ListConsumersFilter) ([]*storage.ConsumerRecord, error) {
	query := `SELECT id, consumer_id, consumer_version, schema_id, schema_version,
			   environment, registered_at, last_validated_at, used_endpoints
		FROM consumers WHERE 1=1`

	args := []interface{}{}

	if filter.SchemaID != "" {
		query += " AND schema_id = ?"
		args = append(args, filter.SchemaID)
	}
	if filter.Environment != "" {
		query += " AND environment = ?"
		args = append(args, filter.Environment)
	}
	if filter.ConsumerID != "" {
		query += " AND consumer_id = ?"
		args = append(args, filter.ConsumerID)
	}

	query += " ORDER BY consumer_id"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list consumers: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var records []*storage.ConsumerRecord
	for rows.Next() {
		var record storage.ConsumerRecord
		var registeredAt, lastValidatedAt string
		var endpointsJSON sql.NullString

		err := rows.Scan(
			&record.ID,
			&record.ConsumerID,
			&record.ConsumerVersion,
			&record.SchemaID,
			&record.SchemaVersion,
			&record.Environment,
			&registeredAt,
			&lastValidatedAt,
			&endpointsJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan consumer: %w", err)
		}

		record.RegisteredAt, _ = time.Parse(time.RFC3339, registeredAt)
		record.LastValidatedAt, _ = time.Parse(time.RFC3339, lastValidatedAt)

		if endpointsJSON.Valid {
			_ = json.Unmarshal([]byte(endpointsJSON.String), &record.UsedEndpoints)
		}

		records = append(records, &record)
	}

	return records, rows.Err()
}

// DeregisterConsumer removes a consumer registration.
func (s *SQLiteStore) DeregisterConsumer(ctx context.Context, consumerID, schemaID, environment string) error {
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM consumers WHERE consumer_id = ? AND schema_id = ? AND environment = ?`,
		consumerID, schemaID, environment)
	if err != nil {
		return fmt.Errorf("failed to deregister consumer: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return &storage.ErrNotFound{Resource: "consumer", ID: consumerID + "/" + schemaID + "/" + environment}
	}

	return nil
}

// UpdateConsumerValidation updates the last validated timestamp for a consumer.
func (s *SQLiteStore) UpdateConsumerValidation(ctx context.Context, consumerID, schemaID, environment string, validatedAt time.Time) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE consumers SET last_validated_at = ? WHERE consumer_id = ? AND schema_id = ? AND environment = ?`,
		validatedAt.Format(time.RFC3339), consumerID, schemaID, environment)
	if err != nil {
		return fmt.Errorf("failed to update consumer validation: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return &storage.ErrNotFound{Resource: "consumer", ID: consumerID + "/" + schemaID + "/" + environment}
	}

	return nil
}
