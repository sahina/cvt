package cvtservice

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Schema registration metrics
	schemasRegistered = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cvt_schemas_registered_total",
			Help: "Total number of schemas registered",
		},
		[]string{"status"}, // success, failure
	)

	schemaRegistrationErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cvt_schema_registration_errors_total",
			Help: "Total number of schema registration errors by type",
		},
		[]string{"error_type"}, // parse_error, validation_error, etc.
	)

	// Validation metrics
	validationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cvt_validations_total",
			Help: "Total number of validations performed",
		},
		[]string{"schema_id", "method", "result"}, // result: valid, invalid, error
	)

	validationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "cvt_validation_duration_seconds",
			Help:    "Duration of validation operations in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0},
		},
		[]string{"schema_id", "method"},
	)

	validationErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cvt_validation_errors_total",
			Help: "Total number of validation errors by category",
		},
		[]string{"error_category"}, // request_invalid, response_invalid, schema_not_found, etc.
	)

	// Cache metrics
	cacheHits = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "cvt_cache_hits_total",
			Help: "Total number of schema cache hits",
		},
	)

	cacheMisses = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "cvt_cache_misses_total",
			Help: "Total number of schema cache misses",
		},
	)

	cacheSize = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "cvt_cache_size_bytes",
			Help: "Current size of the schema cache in bytes",
		},
	)

	cacheItemsCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "cvt_cache_items_total",
			Help: "Current number of items in the schema cache",
		},
	)

	// gRPC metrics
	grpcRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cvt_grpc_requests_total",
			Help: "Total number of gRPC requests",
		},
		[]string{"method", "status"}, // method: RegisterSchema, ValidateInteraction; status: success, failure
	)

	grpcRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "cvt_grpc_request_duration_seconds",
			Help:    "Duration of gRPC requests in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0},
		},
		[]string{"method"},
	)

	// Compatibility and versioning metrics
	breakingChangesDetected = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cvt_breaking_changes_detected_total",
			Help: "Total number of breaking changes detected by type",
		},
		[]string{"change_type"}, // ENDPOINT_REMOVED, REQUIRED_FIELD_ADDED, etc.
	)

	schemaVersionsTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cvt_schema_versions_total",
			Help: "Number of versions per schema",
		},
		[]string{"schema_id"},
	)

	// Authentication metrics
	authSuccessTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "cvt_auth_success_total",
			Help: "Total number of successful authentications",
		},
	)

	authFailureTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cvt_auth_failure_total",
			Help: "Total number of authentication failures by reason",
		},
		[]string{"reason"}, // missing_key, invalid_key
	)

	// Governance metrics
	schemasByOwner = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cvt_schemas_by_owner",
			Help: "Number of schemas per owner",
		},
		[]string{"owner"},
	)

	schemasByTeam = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cvt_schemas_by_team",
			Help: "Number of schemas per team",
		},
		[]string{"team"},
	)

	readOnlyViolations = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "cvt_read_only_violations_total",
			Help: "Total number of attempts to modify read-only schemas",
		},
	)

	// Audit metrics
	auditEventsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cvt_audit_events_total",
			Help: "Total number of audit events by type",
		},
		[]string{"event_type"},
	)
)

// Metrics is a struct that holds all the metrics collectors
type Metrics struct {
	SchemasRegistered        *prometheus.CounterVec
	SchemaRegistrationErrors *prometheus.CounterVec
	ValidationsTotal         *prometheus.CounterVec
	ValidationDuration       *prometheus.HistogramVec
	ValidationErrors         *prometheus.CounterVec
	CacheHits                prometheus.Counter
	CacheMisses              prometheus.Counter
	CacheSize                prometheus.Gauge
	CacheItemsCount          prometheus.Gauge
	GrpcRequestsTotal        *prometheus.CounterVec
	GrpcRequestDuration      *prometheus.HistogramVec
	BreakingChangesDetected  *prometheus.CounterVec
	SchemaVersionsTotal      *prometheus.GaugeVec
	AuthSuccessTotal         prometheus.Counter
	AuthFailureTotal         *prometheus.CounterVec
	SchemasByOwner           *prometheus.GaugeVec
	SchemasByTeam            *prometheus.GaugeVec
	ReadOnlyViolations       prometheus.Counter
	AuditEventsTotal         *prometheus.CounterVec
}

// NewMetrics creates a new Metrics instance
func NewMetrics() *Metrics {
	return &Metrics{
		SchemasRegistered:        schemasRegistered,
		SchemaRegistrationErrors: schemaRegistrationErrors,
		ValidationsTotal:         validationsTotal,
		ValidationDuration:       validationDuration,
		ValidationErrors:         validationErrors,
		CacheHits:                cacheHits,
		CacheMisses:              cacheMisses,
		CacheSize:                cacheSize,
		CacheItemsCount:          cacheItemsCount,
		GrpcRequestsTotal:        grpcRequestsTotal,
		GrpcRequestDuration:      grpcRequestDuration,
		BreakingChangesDetected:  breakingChangesDetected,
		SchemaVersionsTotal:      schemaVersionsTotal,
		AuthSuccessTotal:         authSuccessTotal,
		AuthFailureTotal:         authFailureTotal,
		SchemasByOwner:           schemasByOwner,
		SchemasByTeam:            schemasByTeam,
		ReadOnlyViolations:       readOnlyViolations,
		AuditEventsTotal:         auditEventsTotal,
	}
}
