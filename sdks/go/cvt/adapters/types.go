// Package adapters provides HTTP adapters for automatic contract validation.
//
// This package contains adapters that integrate with Go's standard HTTP
// library to automatically capture and validate HTTP interactions against
// OpenAPI schemas.
//
// Two main adapters are provided:
//
//   - ValidatingRoundTripper: Wraps http.RoundTripper for client-side validation
//   - ValidatingMiddleware: Wraps http.Handler for server-side validation
package adapters

import (
	"context"
	"regexp"

	"github.com/cvt/cvt-sdk/go/cvt"
)

// Validator is the interface that wraps the Validate method.
// This interface allows for easy mocking in tests.
type Validator interface {
	Validate(ctx context.Context, request cvt.ValidationRequest, response cvt.ValidationResponse) (*cvt.ValidationResult, error)
}

// MockingValidator extends Validator with response generation capability.
// This interface is used by MockingRoundTripper to generate mock responses
// from registered OpenAPI schemas without calling real API endpoints.
type MockingValidator interface {
	Validator
	GenerateResponse(ctx context.Context, method, path string, opts *cvt.GenerateOptions) (*cvt.GeneratedResponse, error)
}

// PathFilter can be a string (substring match) or *regexp.Regexp (pattern match).
type PathFilter interface{}

// CapturedInteraction is an alias for cvt.CapturedInteraction.
// This allows interactions captured by adapters to be directly passed to
// validator.RegisterConsumerFromInteractions().
type CapturedInteraction = cvt.CapturedInteraction

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

// shouldValidatePath determines if a path should be validated based on filters.
func shouldValidatePath(path string, includePaths, excludePaths []PathFilter) bool {
	// Check excludes first
	for _, pattern := range excludePaths {
		if matchesPathFilter(path, pattern) {
			return false
		}
	}

	// If includes specified, must match at least one
	if len(includePaths) > 0 {
		for _, pattern := range includePaths {
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
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsImpl(s, substr))
}

func containsImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
