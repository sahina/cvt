package producer

import (
	"sync"
	"time"
)

// Metrics tracks validation statistics.
// This is a simple in-memory implementation.
// For production use, integrate with your preferred metrics system (Prometheus, etc.).
type Metrics struct {
	mu sync.RWMutex

	// Validation counts
	RequestValidations        int64
	RequestValidationsPassed  int64
	RequestValidationsFailed  int64
	ResponseValidations       int64
	ResponseValidationsPassed int64
	ResponseValidationsFailed int64

	// Rejections (Strict mode only)
	RequestsRejected int64

	// Timing (in milliseconds)
	TotalRequestValidationTime  int64
	TotalResponseValidationTime int64
}

// globalMetrics is the default metrics instance.
var globalMetrics = &Metrics{}

// GetMetrics returns the global metrics instance.
func GetMetrics() *Metrics {
	return globalMetrics
}

// ResetMetrics resets all metrics to zero.
func ResetMetrics() {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()

	globalMetrics.RequestValidations = 0
	globalMetrics.RequestValidationsPassed = 0
	globalMetrics.RequestValidationsFailed = 0
	globalMetrics.ResponseValidations = 0
	globalMetrics.ResponseValidationsPassed = 0
	globalMetrics.ResponseValidationsFailed = 0
	globalMetrics.RequestsRejected = 0
	globalMetrics.TotalRequestValidationTime = 0
	globalMetrics.TotalResponseValidationTime = 0
}

// RecordValidation records metrics for a validation result.
func RecordValidation(validationType string, result *ValidationResult) {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()

	switch validationType {
	case "request":
		globalMetrics.RequestValidations++
		if result.Valid {
			globalMetrics.RequestValidationsPassed++
		} else {
			globalMetrics.RequestValidationsFailed++
		}
	case "response":
		globalMetrics.ResponseValidations++
		if result.Valid {
			globalMetrics.ResponseValidationsPassed++
		} else {
			globalMetrics.ResponseValidationsFailed++
		}
	}
}

// recordValidationMetrics is an alias for RecordValidation (for internal use).
func recordValidationMetrics(validationType string, result *ValidationResult) {
	RecordValidation(validationType, result)
}

// RecordRejection records a rejected request.
func RecordRejection() {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.RequestsRejected++
}

// RecordDuration records the duration of a validation.
func RecordDuration(validationType string, duration time.Duration) {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()

	ms := duration.Milliseconds()
	switch validationType {
	case "request":
		globalMetrics.TotalRequestValidationTime += ms
	case "response":
		globalMetrics.TotalResponseValidationTime += ms
	}
}

// Snapshot returns a copy of the current metrics.
func (m *Metrics) Snapshot() Metrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return Metrics{
		RequestValidations:         m.RequestValidations,
		RequestValidationsPassed:   m.RequestValidationsPassed,
		RequestValidationsFailed:   m.RequestValidationsFailed,
		ResponseValidations:        m.ResponseValidations,
		ResponseValidationsPassed:  m.ResponseValidationsPassed,
		ResponseValidationsFailed:  m.ResponseValidationsFailed,
		RequestsRejected:           m.RequestsRejected,
		TotalRequestValidationTime: m.TotalRequestValidationTime,
	}
}

// PrometheusMetrics provides Prometheus-compatible metrics.
// This is optional - only use if you want Prometheus integration.
type PrometheusMetrics struct {
	enabled bool
	prefix  string
}

// NewPrometheusMetrics creates a new PrometheusMetrics instance.
// The prefix is prepended to all metric names (e.g., "cvt_producer_").
func NewPrometheusMetrics(prefix string) *PrometheusMetrics {
	return &PrometheusMetrics{
		enabled: true,
		prefix:  prefix,
	}
}

// MetricName returns the full metric name with prefix.
func (pm *PrometheusMetrics) MetricName(name string) string {
	return pm.prefix + name
}

// Metric definitions for Prometheus integration.
// These are the recommended metric names:
//
// Counter: cvt_producer_validations_total{type="request|response", result="valid|invalid", mode="strict|warn|shadow"}
// Counter: cvt_producer_rejections_total{path="...", method="..."}
// Histogram: cvt_producer_validation_duration_seconds{type="request|response"}
