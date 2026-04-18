package cvt

import (
	"context"
	"sync/atomic"
	"testing"

	eventspb "github.com/sahina/cvt/pkg/cvtplugin/pb/events/v1"
	registrypb "github.com/sahina/cvt/pkg/cvtplugin/pb/registry/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingHooks is a minimal Hooks impl for tests.
type recordingHooks struct {
	validationFailedCalls int32
	lastSchemaID          string
	lastMethod            string
	lastPath              string
	lastErrors            []string
}

func (h *recordingHooks) FetchSchema(_ context.Context, _ *registrypb.FetchSchemaRequest) (*registrypb.FetchSchemaResponse, error) {
	return nil, nil
}
func (h *recordingHooks) RegisterConsumerUsage(_ context.Context, _ *registrypb.RegisterConsumerUsageRequest) (*registrypb.RegisterConsumerUsageResponse, error) {
	return &registrypb.RegisterConsumerUsageResponse{Acknowledged: true}, nil
}
func (h *recordingHooks) OnBreakingChangeDetected(_ context.Context, _ *eventspb.BreakingChangeDetectedRequest) (*eventspb.EventResponse, error) {
	return &eventspb.EventResponse{Acknowledged: true}, nil
}
func (h *recordingHooks) OnValidationFailed(_ context.Context, req *eventspb.ValidationFailedRequest) (*eventspb.EventResponse, error) {
	atomic.AddInt32(&h.validationFailedCalls, 1)
	h.lastSchemaID = req.GetSchemaId()
	h.lastMethod = req.GetMethod()
	h.lastPath = req.GetPath()
	h.lastErrors = nil
	for _, e := range req.GetErrors() {
		h.lastErrors = append(h.lastErrors, e.GetDescription())
	}
	return &eventspb.EventResponse{Acknowledged: true}, nil
}

const tinyOpenAPI = `{
  "openapi": "3.0.0",
  "info": {"title": "Test", "version": "1.0.0"},
  "paths": {
    "/widgets/{id}": {
      "get": {
        "parameters": [
          {"name": "id", "in": "path", "required": true, "schema": {"type": "integer"}}
        ],
        "responses": {
          "200": {
            "description": "ok",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {"id": {"type": "integer"}},
                  "required": ["id"]
                }
              }
            }
          }
        }
      }
    }
  }
}`

func TestOnValidationFailedFiresOnInvalidResponse(t *testing.T) {
	h := &recordingHooks{}
	v := NewValidator()
	v.SetHooks(h)

	err := v.RegisterSchema("widgets", []byte(tinyOpenAPI))
	require.NoError(t, err)

	// Response missing the required `id` field — validator flags invalid,
	// hook fires.
	result, err := v.Validate("widgets", &Interaction{
		Method:          "GET",
		Path:            "/widgets/42",
		Headers:         map[string]string{"Accept": "application/json"},
		StatusCode:      200,
		ResponseHeaders: map[string]string{"Content-Type": "application/json"},
		ResponseBody:    `{}`, // missing "id"
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Valid, "expected validation failure on missing required field; errors=%v", result.Errors)
	assert.Equal(t, int32(1), atomic.LoadInt32(&h.validationFailedCalls))
	assert.Equal(t, "widgets", h.lastSchemaID)
	assert.Equal(t, "GET", h.lastMethod)
	assert.Equal(t, "/widgets/42", h.lastPath)
	assert.NotEmpty(t, h.lastErrors)
}

func TestOnValidationFailedNotFiredOnSuccess(t *testing.T) {
	h := &recordingHooks{}
	v := NewValidator()
	v.SetHooks(h)

	err := v.RegisterSchema("widgets", []byte(tinyOpenAPI))
	require.NoError(t, err)

	result, err := v.Validate("widgets", &Interaction{
		Method:          "GET",
		Path:            "/widgets/42",
		Headers:         map[string]string{"Accept": "application/json"},
		StatusCode:      200,
		ResponseHeaders: map[string]string{"Content-Type": "application/json"},
		ResponseBody:    `{"id": 42}`,
	})
	assert.NoError(t, err)
	assert.True(t, result.Valid)
	assert.Equal(t, int32(0), atomic.LoadInt32(&h.validationFailedCalls))
}

func TestValidatorWithoutHooksFallsBackToNoop(t *testing.T) {
	v := NewValidator()
	// No SetHooks call.
	err := v.RegisterSchema("widgets", []byte(tinyOpenAPI))
	require.NoError(t, err)

	// Invalid response; NoopHooks swallows the event cleanly.
	_, err = v.Validate("widgets", &Interaction{
		Method:          "GET",
		Path:            "/widgets/42",
		StatusCode:      200,
		ResponseHeaders: map[string]string{"Content-Type": "application/json"},
		ResponseBody:    `{}`,
	})
	assert.NoError(t, err)
}
