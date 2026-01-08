// Package producer provides server-side HTTP middleware for validating
// incoming requests and outgoing responses against OpenAPI schemas.
//
// This package enables API producers (servers) to enforce their API contracts
// by validating that:
//   - Incoming requests match the expected schema (path, query, headers, body)
//   - Outgoing responses conform to the documented response schemas
//
// Three validation modes are supported:
//   - Strict: Reject invalid requests with 400/422 errors
//   - Warn: Log violations but allow requests to proceed
//   - Shadow: Validate asynchronously, record metrics only
package producer

import (
	"context"
	"net/http"
	"regexp"
)

// Interaction represents an HTTP request/response pair to validate.
type Interaction struct {
	// Request details
	Method  string
	Path    string
	Headers map[string]string
	Body    string

	// Response details
	StatusCode      int
	ResponseHeaders map[string]string
	ResponseBody    string
}

// Validator is the interface for validating HTTP interactions.
// This can be implemented by either the embedded library or a gRPC client wrapper.
type Validator interface {
	// Validate validates an interaction against a registered schema.
	// Returns a ValidationResult with Valid=true if validation passes.
	Validate(ctx context.Context, schemaID string, interaction *Interaction) (*ValidationResult, error)
}

// ValidationMode determines how validation failures are handled.
type ValidationMode string

const (
	// ModeStrict rejects invalid requests with a 400/422 error response.
	// Invalid responses are logged as errors but cannot be modified.
	ModeStrict ValidationMode = "strict"

	// ModeWarn logs validation failures but allows requests to proceed.
	// Useful for gradual rollout or during migration periods.
	ModeWarn ValidationMode = "warn"

	// ModeShadow validates asynchronously and records metrics only.
	// Never blocks or modifies request/response flow.
	ModeShadow ValidationMode = "shadow"
)

// PathFilter can be a string (substring match) or *regexp.Regexp (pattern match).
type PathFilter interface{}

// Config configures producer-side validation middleware.
type Config struct {
	// SchemaID identifies the schema to validate against (required).
	SchemaID string

	// Validator is the validation backend (required).
	// This can be an embedded validator or a gRPC client wrapper.
	Validator Validator

	// Mode determines how validation failures are handled.
	// Default: ModeStrict
	Mode ValidationMode

	// ValidateRequest enables incoming request validation.
	// Default: true
	ValidateRequest bool

	// ValidateResponse enables outgoing response validation.
	// Default: true
	ValidateResponse bool

	// IncludePaths filters requests to only validate matching paths.
	// If empty, all paths are validated (unless excluded).
	IncludePaths []PathFilter

	// ExcludePaths filters requests to exclude matching paths from validation.
	// Excludes are checked before includes.
	ExcludePaths []PathFilter

	// OnRequestFailure is called when request validation fails.
	// For Strict mode, return true to continue processing, false to reject.
	// For Warn/Shadow modes, this is called for logging/alerting.
	OnRequestFailure func(w http.ResponseWriter, r *http.Request, result *ValidationResult) bool

	// OnResponseFailure is called when response validation fails.
	// Cannot modify the response (already sent), but useful for logging/alerting.
	OnResponseFailure func(r *http.Request, result *ValidationResult)

	// AsyncValidation enables asynchronous validation.
	// Default: false for Strict/Warn, true for Shadow mode.
	AsyncValidation bool
}

// ValidationResult represents the result of a validation operation.
type ValidationResult struct {
	// Valid is true if validation passed.
	Valid bool

	// Errors contains validation error messages.
	Errors []string

	// Type indicates what was validated ("request" or "response").
	Type string
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Mode:             ModeStrict,
		ValidateRequest:  true,
		ValidateResponse: true,
		AsyncValidation:  false,
	}
}

// Validate checks that the configuration is valid.
func (c *Config) Validate() error {
	if c.SchemaID == "" {
		return ErrSchemaIDRequired
	}

	if c.Validator == nil {
		return ErrValidatorRequired
	}

	return nil
}

// ShouldValidateAsync returns whether validation should be async.
func (c *Config) ShouldValidateAsync() bool {
	if c.Mode == ModeShadow {
		return true
	}
	return c.AsyncValidation
}

// matchesPathFilter checks if a path matches a filter pattern.
func matchesPathFilter(path string, pattern PathFilter) bool {
	switch p := pattern.(type) {
	case string:
		return contains(path, p)
	case *regexp.Regexp:
		return p.MatchString(path)
	}
	return false
}

// ShouldValidatePath determines if a path should be validated based on filters.
func (c *Config) ShouldValidatePath(path string) bool {
	// Check excludes first
	for _, pattern := range c.ExcludePaths {
		if matchesPathFilter(path, pattern) {
			return false
		}
	}

	// If includes specified, must match at least one
	if len(c.IncludePaths) > 0 {
		for _, pattern := range c.IncludePaths {
			if matchesPathFilter(path, pattern) {
				return true
			}
		}
		return false
	}

	return true
}

// contains checks if s contains substr.
func contains(s, substr string) bool {
	if substr == "" {
		return true
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
