package cvtservice

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAuditLogger(t *testing.T) {
	t.Run("NewAuditLogger", func(t *testing.T) {
		logger := NewAuditLogger()
		assert.NotNil(t, logger)
		assert.NotNil(t, logger.logger)
	})

	t.Run("Log basic event", func(t *testing.T) {
		logger := NewAuditLogger()

		// This should not panic
		logger.Log(AuditEvent{
			EventType: AuditEventSchemaRegistered,
			SchemaID:  "test-schema",
			Version:   "1.0.0",
			Success:   true,
			Message:   "Test message",
		})
	})

	t.Run("Log with all fields", func(t *testing.T) {
		logger := NewAuditLogger()

		logger.Log(AuditEvent{
			Timestamp: time.Now(),
			EventType: AuditEventSchemaUpdated,
			SchemaID:  "test-schema",
			Version:   "2.0.0",
			Owner:     "john.doe",
			Team:      "platform",
			Success:   true,
			Message:   "Schema updated",
			APIKeyID:  "key-123",
			ClientIP:  "192.168.1.1",
			Method:    "RegisterSchema",
			Duration:  100 * time.Millisecond,
			Details: map[string]interface{}{
				"old_version": "1.0.0",
				"new_version": "2.0.0",
			},
		})
	})

	t.Run("Log failed event", func(t *testing.T) {
		logger := NewAuditLogger()

		logger.Log(AuditEvent{
			EventType: AuditEventAuthFailure,
			Success:   false,
			Message:   "Invalid API key",
			ClientIP:  "10.0.0.1",
		})
	})

	t.Run("LogSchemaRegistered", func(t *testing.T) {
		logger := NewAuditLogger()

		// Should not panic
		logger.LogSchemaRegistered("my-api", "1.0.0", "alice", "api-team", true, "Registered successfully")
	})

	t.Run("LogSchemaUpdated", func(t *testing.T) {
		logger := NewAuditLogger()

		logger.LogSchemaUpdated("my-api", "1.0.0", "2.0.0", "alice", "api-team", true, "Updated successfully")
	})

	t.Run("LogSchemaAccessed", func(t *testing.T) {
		logger := NewAuditLogger()

		logger.LogSchemaAccessed("my-api", "1.0.0", true)
	})

	t.Run("LogSchemaCompared", func(t *testing.T) {
		logger := NewAuditLogger()

		logger.LogSchemaCompared("my-api", "1.0.0", "2.0.0", true, 0)
		logger.LogSchemaCompared("my-api", "1.0.0", "2.0.0", false, 3)
	})

	t.Run("LogValidation", func(t *testing.T) {
		logger := NewAuditLogger()

		logger.LogValidation("my-api", "1.0.0", "GET", "/pets", true, 50*time.Millisecond)
		logger.LogValidation("my-api", "1.0.0", "POST", "/pets", false, 100*time.Millisecond)
	})

	t.Run("LogAuthSuccess", func(t *testing.T) {
		logger := NewAuditLogger()

		logger.LogAuthSuccess("dev-key", "127.0.0.1", "RegisterSchema")
	})

	t.Run("LogAuthFailure", func(t *testing.T) {
		logger := NewAuditLogger()

		logger.LogAuthFailure("10.0.0.1", "ValidateInteraction", "Invalid API key")
	})

	t.Run("LogReadOnlyViolation", func(t *testing.T) {
		logger := NewAuditLogger()

		logger.LogReadOnlyViolation("protected-api", "admin", "core-team")
	})

	t.Run("LogBreakingChange", func(t *testing.T) {
		logger := NewAuditLogger()

		logger.LogBreakingChange("my-api", "1.0.0", "2.0.0", "ENDPOINT_REMOVED", "Endpoint POST /users was removed")
	})
}

func TestGetAuditLogger(t *testing.T) {
	logger1 := GetAuditLogger()
	assert.NotNil(t, logger1)

	logger2 := GetAuditLogger()
	assert.NotNil(t, logger2)

	// Should return same instance
	assert.Same(t, logger1, logger2)
}

func TestAuditEventTypes(t *testing.T) {
	// Verify all event types are defined
	eventTypes := []AuditEventType{
		AuditEventSchemaRegistered,
		AuditEventSchemaUpdated,
		AuditEventSchemaDeleted,
		AuditEventSchemaAccessed,
		AuditEventSchemaCompared,
		AuditEventValidationExecuted,
		AuditEventAuthSuccess,
		AuditEventAuthFailure,
		AuditEventReadOnlyViolation,
		AuditEventBreakingChangeDetected,
	}

	for _, et := range eventTypes {
		assert.NotEmpty(t, string(et))
	}
}
