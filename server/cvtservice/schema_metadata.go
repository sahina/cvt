// Package cvtservice provides schema metadata management for the CVT server.
package cvtservice

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/sahina/cvt/server/pb"
)

// SchemaEntry represents a registered schema with its metadata.
type SchemaEntry struct {
	// Document is the parsed OpenAPI document.
	Document *openapi3.T

	// Content is the original schema content.
	Content string

	// Metadata contains versioning and ownership information.
	Metadata *pb.SchemaMetadata
}

// NewSchemaEntry creates a new SchemaEntry from a parsed document.
func NewSchemaEntry(
	schemaID string,
	content string,
	doc *openapi3.T,
	version string,
	ownership *pb.SchemaOwnership,
) *SchemaEntry {
	now := time.Now().Unix()

	// Compute content hash
	hash := computeSchemaHash(content)

	// Detect OpenAPI version
	openapiVersion := doc.OpenAPI
	if openapiVersion == "" {
		openapiVersion = "3.0.0" // Default if not specified
	}

	// Count endpoints
	endpointCount := countEndpoints(doc)

	metadata := &pb.SchemaMetadata{
		SchemaId:       schemaID,
		SchemaVersion:  version,
		SchemaHash:     hash,
		RegisteredAt:   now,
		UpdatedAt:      now,
		Ownership:      ownership,
		OpenapiVersion: openapiVersion,
		EndpointCount:  int32(endpointCount),
	}

	return &SchemaEntry{
		Document: doc,
		Content:  content,
		Metadata: metadata,
	}
}

// UpdateMetadata updates the entry's metadata with new values.
func (e *SchemaEntry) UpdateMetadata(
	content string,
	doc *openapi3.T,
	version string,
	ownership *pb.SchemaOwnership,
) {
	// Update timestamp
	e.Metadata.UpdatedAt = time.Now().Unix()

	// Update version if provided
	if version != "" {
		e.Metadata.SchemaVersion = version
	}

	// Recompute hash
	e.Metadata.SchemaHash = computeSchemaHash(content)

	// Update OpenAPI version
	if doc.OpenAPI != "" {
		e.Metadata.OpenapiVersion = doc.OpenAPI
	}

	// Recount endpoints
	e.Metadata.EndpointCount = int32(countEndpoints(doc))

	// Update ownership if provided
	if ownership != nil {
		e.Metadata.Ownership = ownership
	}

	// Update document and content
	e.Document = doc
	e.Content = content
}

// computeSchemaHash computes a SHA256 hash of the schema content.
func computeSchemaHash(content string) string {
	// Normalize content by trimming whitespace for consistent hashing
	normalized := strings.TrimSpace(content)
	hash := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(hash[:])
}

// countEndpoints counts the number of endpoints in an OpenAPI document.
// An endpoint is defined as a unique path + method combination.
func countEndpoints(doc *openapi3.T) int {
	if doc == nil || doc.Paths == nil {
		return 0
	}

	count := 0
	for _, pathItem := range doc.Paths.Map() {
		if pathItem == nil {
			continue
		}

		// Count each HTTP method as a separate endpoint
		if pathItem.Get != nil {
			count++
		}
		if pathItem.Post != nil {
			count++
		}
		if pathItem.Put != nil {
			count++
		}
		if pathItem.Delete != nil {
			count++
		}
		if pathItem.Patch != nil {
			count++
		}
		if pathItem.Head != nil {
			count++
		}
		if pathItem.Options != nil {
			count++
		}
	}

	return count
}

// GenerateDefaultVersion generates a version string based on timestamp.
// Format: YYYY.MM.DD-HHMMSS
func GenerateDefaultVersion() string {
	now := time.Now()
	return now.Format("2006.01.02-150405")
}
