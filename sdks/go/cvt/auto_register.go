// Package cvt provides a Go SDK for the Contract Validator Toolkit (CVT).
package cvt

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
)

// CapturedInteraction represents a captured HTTP request/response pair.
// This is used by auto-registration to analyze what endpoints/fields a consumer uses.
type CapturedInteraction struct {
	Request          ValidationRequest
	Response         ValidationResponse
	ValidationResult *ValidationResult
	Timestamp        time.Time
}

// AutoRegisterConfig configures auto-registration of consumers from captured interactions.
type AutoRegisterConfig struct {
	// ConsumerID is the unique consumer identifier (required, e.g., "order-service")
	ConsumerID string

	// ConsumerVersion is the consumer's version (required, e.g., "2.1.0")
	ConsumerVersion string

	// Environment is the deployment environment (required, e.g., "dev", "staging", "prod")
	Environment string

	// SchemaVersion is the schema version being tested against (required, e.g., "1.0.0")
	SchemaVersion string

	// SchemaID overrides auto-extraction from URL hostname (optional).
	// If empty, schemaID is extracted from the mock URL hostname.
	// For example, "http://mock.user-api/users/123" extracts "user-api".
	SchemaID string
}

// BuildConsumerFromInteractions analyzes captured interactions and builds
// registration options without actually registering. Useful for preview/dry-run.
//
// Example:
//
//	mock := adapters.NewMock(validator)
//	// ... run tests ...
//
//	opts, err := validator.BuildConsumerFromInteractions(ctx, mock.GetInteractions(), cvt.AutoRegisterConfig{
//	    ConsumerID:      "order-service",
//	    ConsumerVersion: "2.1.0",
//	    Environment:     "dev",
//	    SchemaVersion:   "1.0.0",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Would register %d endpoints\n", len(opts.UsedEndpoints))
func (v *Validator) BuildConsumerFromInteractions(
	ctx context.Context,
	interactions []CapturedInteraction,
	config AutoRegisterConfig,
) (*RegisterConsumerOptions, error) {
	// Validate required fields
	if config.ConsumerID == "" {
		return nil, fmt.Errorf("consumerID is required")
	}
	if config.ConsumerVersion == "" {
		return nil, fmt.Errorf("consumerVersion is required")
	}
	if config.Environment == "" {
		return nil, fmt.Errorf("environment is required")
	}
	if config.SchemaVersion == "" {
		return nil, fmt.Errorf("schemaVersion is required")
	}

	// Validate interactions
	if len(interactions) == 0 {
		return nil, fmt.Errorf("no interactions to register")
	}

	// Extract schemaID from interactions or use provided override
	schemaID := config.SchemaID
	if schemaID == "" {
		var err error
		schemaID, err = extractSchemaIDFromInteractions(interactions)
		if err != nil {
			return nil, err
		}
	}

	// Merge interactions into endpoint usage
	usedEndpoints := mergeInteractionsToEndpoints(interactions)

	return &RegisterConsumerOptions{
		ConsumerID:      config.ConsumerID,
		ConsumerVersion: config.ConsumerVersion,
		SchemaID:        schemaID,
		SchemaVersion:   config.SchemaVersion,
		Environment:     config.Environment,
		UsedEndpoints:   usedEndpoints,
	}, nil
}

// RegisterConsumerFromInteractions registers a consumer based on captured mock interactions.
// This combines BuildConsumerFromInteractions + RegisterConsumer.
//
// Example:
//
//	mock := adapters.NewMock(validator)
//	client := mock.Client()
//
//	// Run tests
//	resp, _ := client.Get("http://mock.user-api/users/123")
//	resp2, _ := client.Post("http://mock.user-api/users", "application/json", body)
//
//	// Auto-register consumer from captured interactions
//	info, err := validator.RegisterConsumerFromInteractions(ctx, mock.GetInteractions(), cvt.AutoRegisterConfig{
//	    ConsumerID:      "order-service",
//	    ConsumerVersion: "2.1.0",
//	    Environment:     "dev",
//	    SchemaVersion:   "1.0.0",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Registered %s with %d endpoints\n", info.ConsumerID, len(info.UsedEndpoints))
func (v *Validator) RegisterConsumerFromInteractions(
	ctx context.Context,
	interactions []CapturedInteraction,
	config AutoRegisterConfig,
) (*ConsumerInfo, error) {
	opts, err := v.BuildConsumerFromInteractions(ctx, interactions, config)
	if err != nil {
		return nil, err
	}

	return v.RegisterConsumer(ctx, *opts)
}

// extractSchemaIDFromInteractions extracts the schemaID from captured interactions.
// It looks at the request URLs and extracts the hostname, stripping "mock." prefix if present.
// Returns an error if multiple different schemaIDs are detected.
func extractSchemaIDFromInteractions(interactions []CapturedInteraction) (string, error) {
	schemaIDs := make(map[string]struct{})

	for _, interaction := range interactions {
		// The Path in ValidationRequest might be just the path, or it might be a full URL
		// Check the interaction's request for URL information
		path := interaction.Request.Path

		// If path starts with http:// or https://, extract hostname
		if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
			schemaID, err := extractSchemaIDFromURL(path)
			if err != nil {
				continue // Skip malformed URLs
			}
			schemaIDs[schemaID] = struct{}{}
		}
	}

	if len(schemaIDs) == 0 {
		return "", fmt.Errorf("could not extract schemaID from interactions; provide SchemaID in config")
	}

	if len(schemaIDs) > 1 {
		ids := make([]string, 0, len(schemaIDs))
		for id := range schemaIDs {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		return "", fmt.Errorf("multiple schemas detected (%s); provide explicit SchemaID in config", strings.Join(ids, ", "))
	}

	// Return the single schemaID
	for id := range schemaIDs {
		return id, nil
	}

	return "", fmt.Errorf("unexpected error extracting schemaID")
}

// extractSchemaIDFromURL extracts the schemaID from a mock URL.
// For example: "http://mock.user-api/users/123" returns "user-api"
func extractSchemaIDFromURL(urlStr string) (string, error) {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	host := parsed.Hostname()
	if host == "" {
		return "", fmt.Errorf("URL has no hostname")
	}

	// Strip "mock." prefix if present
	if strings.HasPrefix(host, "mock.") {
		return strings.TrimPrefix(host, "mock."), nil
	}

	return host, nil
}

// mergeInteractionsToEndpoints converts captured interactions to endpoint usage,
// deduplicating by method+path and merging usedFields.
func mergeInteractionsToEndpoints(interactions []CapturedInteraction) []EndpointUsage {
	// Key: "METHOD:/path"
	endpointMap := make(map[string]*EndpointUsage)

	for _, interaction := range interactions {
		path := normalizePathForEndpoint(interaction.Request.Path)
		key := interaction.Request.Method + ":" + path

		fields := extractFieldsFromBody(interaction.Response.Body, "")

		if existing, ok := endpointMap[key]; ok {
			// Merge fields (union)
			existing.UsedFields = mergeStringSlices(existing.UsedFields, fields)
		} else {
			endpointMap[key] = &EndpointUsage{
				Method:     interaction.Request.Method,
				Path:       path,
				UsedFields: fields,
			}
		}
	}

	// Convert map to sorted slice for deterministic output
	result := make([]EndpointUsage, 0, len(endpointMap))
	keys := make([]string, 0, len(endpointMap))
	for k := range endpointMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		ep := endpointMap[k]
		// Sort UsedFields for deterministic output
		sort.Strings(ep.UsedFields)
		result = append(result, *ep)
	}

	return result
}

// normalizePathForEndpoint extracts and normalizes the path from a URL.
// Removes query string and extracts path from full URLs.
func normalizePathForEndpoint(pathOrURL string) string {
	// If it's a full URL, parse it to extract just the path
	if strings.HasPrefix(pathOrURL, "http://") || strings.HasPrefix(pathOrURL, "https://") {
		parsed, err := url.Parse(pathOrURL)
		if err == nil {
			pathOrURL = parsed.Path
		}
	}

	// Remove query string if present
	if idx := strings.Index(pathOrURL, "?"); idx != -1 {
		pathOrURL = pathOrURL[:idx]
	}

	return pathOrURL
}

// extractFieldsFromBody recursively extracts all field paths from a JSON body.
// Uses dot notation for nested fields (e.g., "user.address.city").
func extractFieldsFromBody(body any, prefix string) []string {
	if body == nil {
		return nil
	}

	var fields []string

	switch v := body.(type) {
	case map[string]any:
		for key, value := range v {
			fieldPath := key
			if prefix != "" {
				fieldPath = prefix + "." + key
			}
			fields = append(fields, fieldPath)
			// Recursively extract nested fields
			fields = append(fields, extractFieldsFromBody(value, fieldPath)...)
		}
	case []any:
		// For arrays, extract fields from the first element as representative
		if len(v) > 0 {
			fields = append(fields, extractFieldsFromBody(v[0], prefix)...)
		}
	}

	return fields
}

// mergeStringSlices merges two string slices, removing duplicates.
func mergeStringSlices(a, b []string) []string {
	seen := make(map[string]struct{})
	for _, s := range a {
		seen[s] = struct{}{}
	}
	for _, s := range b {
		seen[s] = struct{}{}
	}

	result := make([]string, 0, len(seen))
	for s := range seen {
		result = append(result, s)
	}
	return result
}
