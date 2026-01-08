// Package storage provides the persistence layer abstraction for CVT.
package storage

import (
	"context"
	"time"

	"github.com/cvt/cvt/server/pb"
	"github.com/getkin/kin-openapi/openapi3"
)

// SchemaRecord represents a stored schema with all metadata.
type SchemaRecord struct {
	ID             string              // Internal UUID
	SchemaID       string              // User-facing identifier (e.g., "user-service")
	Version        string              // Semantic version (e.g., "1.2.3")
	Content        string              // Raw OpenAPI content (JSON/YAML)
	ContentHash    string              // SHA256 hash of normalized content
	Document       *openapi3.T         // Parsed document (transient, not stored)
	OpenAPIVersion string              // Detected OpenAPI version (e.g., "3.0.0")
	EndpointCount  int32               // Number of endpoints in schema
	IsLatest       bool                // True if this is the latest version
	RegisteredAt   time.Time           // Creation timestamp
	UpdatedAt      time.Time           // Last update timestamp
	Ownership      *pb.SchemaOwnership // Ownership information
	Environment    string              // Environment tag (dev, staging, prod)
}

// ValidationRecord represents a stored validation run.
type ValidationRecord struct {
	ID              string
	SchemaID        string
	SchemaVersion   string
	SchemaHash      string
	RequestMethod   string
	RequestPath     string
	RequestHeaders  map[string]string
	RequestBody     string
	ResponseStatus  int32
	ResponseHeaders map[string]string
	ResponseBody    string
	Valid           bool
	Errors          []string
	DurationMs      int64
	ValidatedAt     time.Time
	Environment     string
	ClientID        string
	ClientIP        string
}

// ComparisonRecord represents a stored schema comparison.
type ComparisonRecord struct {
	ID              string
	SchemaID        string
	OldVersion      string
	NewVersion      string
	Compatible      bool
	BreakingChanges []*pb.BreakingChange
	ComparedAt      time.Time
}

// ConsumerRecord represents a stored consumer registration.
type ConsumerRecord struct {
	ID              string          // Internal UUID
	ConsumerID      string          // User-facing identifier (e.g., "order-service")
	ConsumerVersion string          // Consumer's version (e.g., "2.1.0")
	SchemaID        string          // Schema this consumer depends on
	SchemaVersion   string          // Schema version consumer was tested against
	Environment     string          // Environment (dev, staging, prod)
	RegisteredAt    time.Time       // Initial registration timestamp
	LastValidatedAt time.Time       // Last successful validation timestamp
	UsedEndpoints   []EndpointUsage // Which endpoints the consumer uses
}

// EndpointUsage describes which endpoints and fields a consumer uses.
type EndpointUsage struct {
	Method     string   // HTTP method (GET, POST, etc.)
	Path       string   // API path (e.g., "/users/{id}")
	UsedFields []string // Fields used in response (e.g., ["email", "name"])
}

// ListConsumersFilter provides filtering options for consumer queries.
type ListConsumersFilter struct {
	SchemaID    string // Filter by schema ID
	Environment string // Filter by environment
	ConsumerID  string // Filter by consumer ID
}

// ListSchemasFilter provides filtering options for schema queries.
type ListSchemasFilter struct {
	Owner       string
	Team        string
	Environment string
	PageSize    int32
	PageToken   string
}

// ListValidationsFilter provides filtering options for validation queries.
type ListValidationsFilter struct {
	SchemaID    string
	Method      string
	Environment string
	Valid       *bool // nil = all, true = valid only, false = invalid only
	StartTime   time.Time
	EndTime     time.Time
	PageSize    int32
	PageToken   string
}

// ValidationAnalytics provides aggregated validation statistics.
type ValidationAnalytics struct {
	TotalValidations int64
	PassCount        int64
	FailCount        int64
	PassRate         float64
	TopErrors        []ErrorCount
	ByMethod         map[string]int64
	BySchema         map[string]int64
}

// ErrorCount represents an error with its occurrence count.
type ErrorCount struct {
	Error string
	Count int64
}

// Store defines the persistence layer interface for CVT.
// All implementations must be thread-safe.
type Store interface {
	// Schema operations
	SetSchema(ctx context.Context, record *SchemaRecord) error
	GetSchema(ctx context.Context, schemaID string) (*SchemaRecord, error)
	GetSchemaVersion(ctx context.Context, schemaID, version string) (*SchemaRecord, error)
	DeleteSchema(ctx context.Context, schemaID string) error
	DeleteSchemaVersion(ctx context.Context, schemaID, version string) error
	ListSchemaIDs(ctx context.Context) ([]string, error)
	ListVersions(ctx context.Context, schemaID string) ([]string, error)
	ListSchemas(ctx context.Context, filter ListSchemasFilter) (schemas []*SchemaRecord, nextPageToken string, totalCount int32, err error)
	GetPreviousVersion(ctx context.Context, schemaID, currentVersion string) (string, error)

	// Validation run operations
	RecordValidation(ctx context.Context, record *ValidationRecord) error
	ListValidations(ctx context.Context, filter ListValidationsFilter) ([]*ValidationRecord, string, error)
	GetValidationAnalytics(ctx context.Context, filter ListValidationsFilter) (*ValidationAnalytics, error)

	// Comparison operations
	RecordComparison(ctx context.Context, record *ComparisonRecord) error
	GetComparison(ctx context.Context, schemaID, oldVersion, newVersion string) (*ComparisonRecord, error)

	// Consumer registry operations
	RegisterConsumer(ctx context.Context, record *ConsumerRecord) error
	GetConsumer(ctx context.Context, consumerID, schemaID, environment string) (*ConsumerRecord, error)
	ListConsumers(ctx context.Context, filter ListConsumersFilter) ([]*ConsumerRecord, error)
	DeregisterConsumer(ctx context.Context, consumerID, schemaID, environment string) error
	UpdateConsumerValidation(ctx context.Context, consumerID, schemaID, environment string, validatedAt time.Time) error

	// Lifecycle
	Migrate(ctx context.Context) error
	Close() error
	Ping(ctx context.Context) error
}

// ErrNotFound is returned when a requested record does not exist.
type ErrNotFound struct {
	Resource string
	ID       string
}

func (e *ErrNotFound) Error() string {
	return e.Resource + " not found: " + e.ID
}

// IsNotFound checks if the error is an ErrNotFound.
func IsNotFound(err error) bool {
	_, ok := err.(*ErrNotFound)
	return ok
}
