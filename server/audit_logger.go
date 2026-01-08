// Package main provides audit logging functionality for the CVT server.
// This file implements structured audit logging for schema operations.
package main

import (
	"context"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
)

// AuditEventType represents the type of audit event.
type AuditEventType string

const (
	// Schema lifecycle events
	AuditEventSchemaRegistered   AuditEventType = "SCHEMA_REGISTERED"
	AuditEventSchemaUpdated      AuditEventType = "SCHEMA_UPDATED"
	AuditEventSchemaDeleted      AuditEventType = "SCHEMA_DELETED"
	AuditEventSchemaAccessed     AuditEventType = "SCHEMA_ACCESSED"
	AuditEventSchemaCompared     AuditEventType = "SCHEMA_COMPARED"
	AuditEventValidationExecuted AuditEventType = "VALIDATION_EXECUTED"

	// Security events
	AuditEventAuthSuccess       AuditEventType = "AUTH_SUCCESS"
	AuditEventAuthFailure       AuditEventType = "AUTH_FAILURE"
	AuditEventReadOnlyViolation AuditEventType = "READ_ONLY_VIOLATION"

	// Breaking change events
	AuditEventBreakingChangeDetected AuditEventType = "BREAKING_CHANGE_DETECTED"
)

// AuditEvent represents an auditable event in the system.
type AuditEvent struct {
	// Timestamp when the event occurred
	Timestamp time.Time `json:"timestamp"`

	// EventType identifies the type of audit event
	EventType AuditEventType `json:"event_type"`

	// SchemaID is the identifier of the schema involved (if applicable)
	SchemaID string `json:"schema_id,omitempty"`

	// Version is the schema version involved (if applicable)
	Version string `json:"version,omitempty"`

	// Owner is the owner of the schema (if applicable)
	Owner string `json:"owner,omitempty"`

	// Team is the team responsible for the schema (if applicable)
	Team string `json:"team,omitempty"`

	// Success indicates whether the operation was successful
	Success bool `json:"success"`

	// Message provides additional context about the event
	Message string `json:"message,omitempty"`

	// APIKeyID is the identifier of the API key used (if applicable)
	APIKeyID string `json:"api_key_id,omitempty"`

	// ClientIP is the IP address of the client (if available)
	ClientIP string `json:"client_ip,omitempty"`

	// Method is the gRPC method invoked
	Method string `json:"method,omitempty"`

	// Duration is how long the operation took
	Duration time.Duration `json:"duration,omitempty"`

	// Details contains additional structured data about the event
	Details map[string]interface{} `json:"details,omitempty"`
}

// AuditLogger handles audit event logging.
type AuditLogger struct {
	logger *zap.Logger
}

// NewAuditLogger creates a new AuditLogger instance.
func NewAuditLogger() *AuditLogger {
	// Get the logger, initializing if needed
	log := logger
	if log == nil {
		// Create a development logger for tests
		var err error
		log, err = zap.NewDevelopment()
		if err != nil {
			log = zap.NewNop()
		}
	}
	return &AuditLogger{
		logger: log.Named("audit"),
	}
}

// Log writes an audit event to the log.
func (a *AuditLogger) Log(event AuditEvent) {
	// Ensure timestamp is set
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Build log fields
	fields := []zap.Field{
		zap.String("event_type", string(event.EventType)),
		zap.Time("timestamp", event.Timestamp),
		zap.Bool("success", event.Success),
	}

	if event.SchemaID != "" {
		fields = append(fields, zap.String("schema_id", event.SchemaID))
	}
	if event.Version != "" {
		fields = append(fields, zap.String("version", event.Version))
	}
	if event.Owner != "" {
		fields = append(fields, zap.String("owner", event.Owner))
	}
	if event.Team != "" {
		fields = append(fields, zap.String("team", event.Team))
	}
	if event.Message != "" {
		fields = append(fields, zap.String("message", event.Message))
	}
	if event.APIKeyID != "" {
		fields = append(fields, zap.String("api_key_id", event.APIKeyID))
	}
	if event.ClientIP != "" {
		fields = append(fields, zap.String("client_ip", event.ClientIP))
	}
	if event.Method != "" {
		fields = append(fields, zap.String("method", event.Method))
	}
	if event.Duration > 0 {
		fields = append(fields, zap.Duration("duration", event.Duration))
	}
	if len(event.Details) > 0 {
		fields = append(fields, zap.Any("details", event.Details))
	}

	// Log at appropriate level based on success
	if event.Success {
		a.logger.Info("Audit event", fields...)
	} else {
		a.logger.Warn("Audit event", fields...)
	}
}

// LogSchemaRegistered logs a schema registration event.
func (a *AuditLogger) LogSchemaRegistered(schemaID, version, owner, team string, success bool, message string) {
	a.Log(AuditEvent{
		EventType: AuditEventSchemaRegistered,
		SchemaID:  schemaID,
		Version:   version,
		Owner:     owner,
		Team:      team,
		Success:   success,
		Message:   message,
	})
}

// LogSchemaUpdated logs a schema update event.
func (a *AuditLogger) LogSchemaUpdated(schemaID, oldVersion, newVersion, owner, team string, success bool, message string) {
	a.Log(AuditEvent{
		EventType: AuditEventSchemaUpdated,
		SchemaID:  schemaID,
		Version:   newVersion,
		Owner:     owner,
		Team:      team,
		Success:   success,
		Message:   message,
		Details: map[string]interface{}{
			"old_version": oldVersion,
			"new_version": newVersion,
		},
	})
}

// LogSchemaAccessed logs a schema access event.
func (a *AuditLogger) LogSchemaAccessed(schemaID, version string, success bool) {
	a.Log(AuditEvent{
		EventType: AuditEventSchemaAccessed,
		SchemaID:  schemaID,
		Version:   version,
		Success:   success,
	})
}

// LogSchemaCompared logs a schema comparison event.
func (a *AuditLogger) LogSchemaCompared(schemaID, oldVersion, newVersion string, compatible bool, breakingChangesCount int) {
	a.Log(AuditEvent{
		EventType: AuditEventSchemaCompared,
		SchemaID:  schemaID,
		Success:   true,
		Details: map[string]interface{}{
			"old_version":            oldVersion,
			"new_version":            newVersion,
			"compatible":             compatible,
			"breaking_changes_count": breakingChangesCount,
		},
	})
}

// LogValidation logs a validation event.
func (a *AuditLogger) LogValidation(schemaID, version, method, path string, valid bool, duration time.Duration) {
	a.Log(AuditEvent{
		EventType: AuditEventValidationExecuted,
		SchemaID:  schemaID,
		Version:   version,
		Success:   valid,
		Duration:  duration,
		Details: map[string]interface{}{
			"http_method": method,
			"http_path":   path,
		},
	})
}

// LogAuthSuccess logs a successful authentication.
func (a *AuditLogger) LogAuthSuccess(apiKeyID, clientIP, method string) {
	a.Log(AuditEvent{
		EventType: AuditEventAuthSuccess,
		APIKeyID:  apiKeyID,
		ClientIP:  clientIP,
		Method:    method,
		Success:   true,
	})
}

// LogAuthFailure logs a failed authentication attempt.
func (a *AuditLogger) LogAuthFailure(clientIP, method, reason string) {
	a.Log(AuditEvent{
		EventType: AuditEventAuthFailure,
		ClientIP:  clientIP,
		Method:    method,
		Success:   false,
		Message:   reason,
	})
}

// LogReadOnlyViolation logs an attempt to modify a read-only schema.
func (a *AuditLogger) LogReadOnlyViolation(schemaID, owner, team string) {
	a.Log(AuditEvent{
		EventType: AuditEventReadOnlyViolation,
		SchemaID:  schemaID,
		Owner:     owner,
		Team:      team,
		Success:   false,
		Message:   "Attempted to modify read-only schema",
	})

	// Increment metric
	readOnlyViolations.Inc()
}

// LogBreakingChange logs a breaking change detection event.
func (a *AuditLogger) LogBreakingChange(schemaID, oldVersion, newVersion, changeType, description string) {
	a.Log(AuditEvent{
		EventType: AuditEventBreakingChangeDetected,
		SchemaID:  schemaID,
		Success:   true,
		Details: map[string]interface{}{
			"old_version": oldVersion,
			"new_version": newVersion,
			"change_type": changeType,
			"description": description,
		},
	})
}

// ExtractClientInfo extracts client information from the gRPC context.
func ExtractClientInfo(ctx context.Context) (apiKeyID, clientIP string) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", ""
	}

	// Get API key ID from metadata
	if keys := md.Get(APIKeyMetadataKey); len(keys) > 0 {
		apiKeyID = keys[0]
	}

	// Get client IP from metadata (commonly set by load balancers)
	if ips := md.Get("x-forwarded-for"); len(ips) > 0 {
		clientIP = ips[0]
	} else if ips := md.Get("x-real-ip"); len(ips) > 0 {
		clientIP = ips[0]
	}

	return apiKeyID, clientIP
}

// Global audit logger instance
var auditLogger *AuditLogger

// InitAuditLogger initializes the global audit logger.
func InitAuditLogger() {
	auditLogger = NewAuditLogger()
}

// GetAuditLogger returns the global audit logger.
func GetAuditLogger() *AuditLogger {
	if auditLogger == nil {
		auditLogger = NewAuditLogger()
	}
	return auditLogger
}
