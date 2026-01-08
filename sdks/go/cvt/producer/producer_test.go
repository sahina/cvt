package producer

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockValidator implements the Validator interface for testing.
type MockValidator struct {
	// ValidateFunc is called when Validate is invoked.
	ValidateFunc func(ctx context.Context, schemaID string, interaction *Interaction) (*ValidationResult, error)

	// LastInteraction stores the last interaction passed to Validate.
	LastInteraction *Interaction
}

// Validate implements the Validator interface.
func (m *MockValidator) Validate(ctx context.Context, schemaID string, interaction *Interaction) (*ValidationResult, error) {
	m.LastInteraction = interaction
	if m.ValidateFunc != nil {
		return m.ValidateFunc(ctx, schemaID, interaction)
	}
	return &ValidationResult{Valid: true}, nil
}

// AlwaysValidValidator returns a validator that always returns valid.
func AlwaysValidValidator() *MockValidator {
	return &MockValidator{
		ValidateFunc: func(ctx context.Context, schemaID string, interaction *Interaction) (*ValidationResult, error) {
			return &ValidationResult{Valid: true}, nil
		},
	}
}

// AlwaysInvalidValidator returns a validator that always returns invalid with errors.
func AlwaysInvalidValidator(errors []string) *MockValidator {
	return &MockValidator{
		ValidateFunc: func(ctx context.Context, schemaID string, interaction *Interaction) (*ValidationResult, error) {
			return &ValidationResult{Valid: false, Errors: errors}, nil
		},
	}
}

func TestNewProducer(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		p, err := NewProducer(Config{
			SchemaID:  "test-schema",
			Validator: AlwaysValidValidator(),
		})
		require.NoError(t, err)
		assert.NotNil(t, p)
	})

	t.Run("missing schema ID", func(t *testing.T) {
		p, err := NewProducer(Config{
			Validator: AlwaysValidValidator(),
		})
		assert.ErrorIs(t, err, ErrSchemaIDRequired)
		assert.Nil(t, p)
	})

	t.Run("missing validator", func(t *testing.T) {
		p, err := NewProducer(Config{
			SchemaID: "test-schema",
		})
		assert.ErrorIs(t, err, ErrValidatorRequired)
		assert.Nil(t, p)
	})

	t.Run("defaults applied", func(t *testing.T) {
		p, err := NewProducer(Config{
			SchemaID:  "test-schema",
			Validator: AlwaysValidValidator(),
		})
		require.NoError(t, err)
		assert.Equal(t, ModeStrict, p.config.Mode)
		assert.True(t, p.config.ValidateRequest)
		assert.True(t, p.config.ValidateResponse)
	})
}

func TestValidateRequest(t *testing.T) {
	t.Run("valid request", func(t *testing.T) {
		validator := AlwaysValidValidator()
		p, err := NewProducer(Config{
			SchemaID:  "test-schema",
			Validator: validator,
		})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"test"}`))
		req.Header.Set("Content-Type", "application/json")

		result := p.ValidateRequest(context.Background(), req, []byte(`{"name":"test"}`))

		assert.True(t, result.Valid)
		assert.Empty(t, result.Errors)
		assert.Equal(t, "request", result.Type)

		// Check interaction was passed correctly
		assert.Equal(t, http.MethodPost, validator.LastInteraction.Method)
		assert.Equal(t, "/users", validator.LastInteraction.Path)
		assert.Equal(t, `{"name":"test"}`, validator.LastInteraction.Body)
	})

	t.Run("invalid request", func(t *testing.T) {
		validator := AlwaysInvalidValidator([]string{"missing required field: email"})
		p, err := NewProducer(Config{
			SchemaID:  "test-schema",
			Validator: validator,
		})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"name":"test"}`))
		result := p.ValidateRequest(context.Background(), req, []byte(`{"name":"test"}`))

		assert.False(t, result.Valid)
		assert.Contains(t, result.Errors, "missing required field: email")
	})

	t.Run("validation disabled", func(t *testing.T) {
		validator := AlwaysInvalidValidator([]string{"should not be called"})
		p, err := NewProducer(Config{
			SchemaID:         "test-schema",
			Validator:        validator,
			ValidateRequest:  false,
			ValidateResponse: true,
		})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/users", nil)
		result := p.ValidateRequest(context.Background(), req, nil)

		assert.True(t, result.Valid)
		assert.Nil(t, validator.LastInteraction) // Validator should not be called
	})
}

func TestValidateResponse(t *testing.T) {
	t.Run("valid response", func(t *testing.T) {
		validator := AlwaysValidValidator()
		p, err := NewProducer(Config{
			SchemaID:  "test-schema",
			Validator: validator,
		})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
		respBody := []byte(`{"id":123,"name":"test"}`)

		result := p.ValidateResponse(
			context.Background(),
			req,
			nil,
			http.StatusOK,
			http.Header{"Content-Type": []string{"application/json"}},
			respBody,
		)

		assert.True(t, result.Valid)
		assert.Equal(t, "response", result.Type)

		// Check interaction
		assert.Equal(t, http.StatusOK, validator.LastInteraction.StatusCode)
		assert.Equal(t, string(respBody), validator.LastInteraction.ResponseBody)
	})

	t.Run("invalid response", func(t *testing.T) {
		validator := AlwaysInvalidValidator([]string{"response body type mismatch"})
		p, err := NewProducer(Config{
			SchemaID:  "test-schema",
			Validator: validator,
		})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
		result := p.ValidateResponse(
			context.Background(),
			req,
			nil,
			http.StatusOK,
			nil,
			[]byte(`{"id":"not-a-number"}`),
		)

		assert.False(t, result.Valid)
		assert.Contains(t, result.Errors, "response body type mismatch")
	})
}

func TestHandleRequestValidationFailure(t *testing.T) {
	t.Run("strict mode rejects", func(t *testing.T) {
		p, err := NewProducer(Config{
			SchemaID:  "test-schema",
			Validator: AlwaysValidValidator(),
			Mode:      ModeStrict,
		})
		require.NoError(t, err)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/users", nil)
		result := &ValidationResult{Valid: false, Errors: []string{"test error"}}

		continueProcessing := p.HandleRequestValidationFailure(w, req, result)

		assert.False(t, continueProcessing)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]any
		err = json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, "Request validation failed", response["error"])
	})

	t.Run("warn mode continues", func(t *testing.T) {
		p, err := NewProducer(Config{
			SchemaID:  "test-schema",
			Validator: AlwaysValidValidator(),
			Mode:      ModeWarn,
		})
		require.NoError(t, err)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/users", nil)
		result := &ValidationResult{Valid: false, Errors: []string{"test error"}}

		continueProcessing := p.HandleRequestValidationFailure(w, req, result)

		assert.True(t, continueProcessing)
		// Should not write to response
		assert.Equal(t, 0, w.Body.Len())
	})

	t.Run("shadow mode continues", func(t *testing.T) {
		p, err := NewProducer(Config{
			SchemaID:  "test-schema",
			Validator: AlwaysValidValidator(),
			Mode:      ModeShadow,
		})
		require.NoError(t, err)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/users", nil)
		result := &ValidationResult{Valid: false, Errors: []string{"test error"}}

		continueProcessing := p.HandleRequestValidationFailure(w, req, result)

		assert.True(t, continueProcessing)
	})

	t.Run("custom handler", func(t *testing.T) {
		customHandlerCalled := false
		p, err := NewProducer(Config{
			SchemaID:  "test-schema",
			Validator: AlwaysValidValidator(),
			Mode:      ModeStrict,
			OnRequestFailure: func(w http.ResponseWriter, r *http.Request, result *ValidationResult) bool {
				customHandlerCalled = true
				w.WriteHeader(http.StatusUnprocessableEntity)
				return false
			},
		})
		require.NoError(t, err)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/users", nil)
		result := &ValidationResult{Valid: false, Errors: []string{"test error"}}

		continueProcessing := p.HandleRequestValidationFailure(w, req, result)

		assert.True(t, customHandlerCalled)
		assert.False(t, continueProcessing)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})
}

func TestPathFiltering(t *testing.T) {
	t.Run("include paths", func(t *testing.T) {
		config := Config{
			SchemaID:     "test-schema",
			Validator:    AlwaysValidValidator(),
			IncludePaths: []PathFilter{"/api/", "/users"},
		}

		assert.True(t, config.ShouldValidatePath("/api/users"))
		assert.True(t, config.ShouldValidatePath("/users/123"))
		assert.False(t, config.ShouldValidatePath("/health"))
		assert.False(t, config.ShouldValidatePath("/metrics"))
	})

	t.Run("exclude paths", func(t *testing.T) {
		config := Config{
			SchemaID:     "test-schema",
			Validator:    AlwaysValidValidator(),
			ExcludePaths: []PathFilter{"/health", "/metrics"},
		}

		assert.True(t, config.ShouldValidatePath("/api/users"))
		assert.True(t, config.ShouldValidatePath("/users/123"))
		assert.False(t, config.ShouldValidatePath("/health"))
		assert.False(t, config.ShouldValidatePath("/metrics"))
	})

	t.Run("exclude takes precedence", func(t *testing.T) {
		config := Config{
			SchemaID:     "test-schema",
			Validator:    AlwaysValidValidator(),
			IncludePaths: []PathFilter{"/api/"},
			ExcludePaths: []PathFilter{"/api/internal"},
		}

		assert.True(t, config.ShouldValidatePath("/api/users"))
		assert.False(t, config.ShouldValidatePath("/api/internal/debug"))
	})
}

func TestResponseCapture(t *testing.T) {
	t.Run("captures status and body", func(t *testing.T) {
		w := httptest.NewRecorder()
		capture := NewResponseCapture(w)

		capture.WriteHeader(http.StatusCreated)
		n, err := capture.Write([]byte(`{"id":1}`))

		assert.NoError(t, err)
		assert.Equal(t, 8, n)
		assert.Equal(t, http.StatusCreated, capture.StatusCode)
		assert.Equal(t, `{"id":1}`, capture.Body.String())

		// Also written to underlying writer
		assert.Equal(t, http.StatusCreated, w.Code)
		assert.Equal(t, `{"id":1}`, w.Body.String())
	})

	t.Run("default status code", func(t *testing.T) {
		w := httptest.NewRecorder()
		capture := NewResponseCapture(w)

		_, _ = capture.Write([]byte("data"))

		assert.Equal(t, http.StatusOK, capture.StatusCode)
	})
}

func TestCaptureRequestBody(t *testing.T) {
	t.Run("captures and restores body", func(t *testing.T) {
		original := `{"test":"data"}`
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(original))

		body, err := CaptureRequestBody(req)

		assert.NoError(t, err)
		assert.Equal(t, original, string(body))

		// Body should be restored
		restored, err := io.ReadAll(req.Body)
		assert.NoError(t, err)
		assert.Equal(t, original, string(restored))
	})

	t.Run("nil body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Body = nil

		body, err := CaptureRequestBody(req)

		assert.NoError(t, err)
		assert.Nil(t, body)
	})
}

func TestMetrics(t *testing.T) {
	ResetMetrics()

	t.Run("record validation", func(t *testing.T) {
		RecordValidation("request", &ValidationResult{Valid: true})
		RecordValidation("request", &ValidationResult{Valid: false})
		RecordValidation("response", &ValidationResult{Valid: true})

		metrics := GetMetrics().Snapshot()
		assert.Equal(t, int64(2), metrics.RequestValidations)
		assert.Equal(t, int64(1), metrics.RequestValidationsPassed)
		assert.Equal(t, int64(1), metrics.RequestValidationsFailed)
		assert.Equal(t, int64(1), metrics.ResponseValidations)
		assert.Equal(t, int64(1), metrics.ResponseValidationsPassed)
	})

	t.Run("record rejection", func(t *testing.T) {
		ResetMetrics()
		RecordRejection()
		RecordRejection()

		metrics := GetMetrics().Snapshot()
		assert.Equal(t, int64(2), metrics.RequestsRejected)
	})
}

// BenchmarkValidateRequest benchmarks request validation.
func BenchmarkValidateRequest(b *testing.B) {
	p, _ := NewProducer(Config{
		SchemaID:  "test-schema",
		Validator: AlwaysValidValidator(),
	})

	req := httptest.NewRequest(http.MethodPost, "/users", nil)
	body := []byte(`{"name":"test","email":"test@example.com"}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.ValidateRequest(context.Background(), req, body)
	}
}
