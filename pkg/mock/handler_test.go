package mock

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sahina/cvt/pkg/cvt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSchema = `{
  "openapi": "3.0.0",
  "info": {"title": "Test API", "version": "1.0.0"},
  "paths": {
    "/users": {
      "get": {
        "operationId": "listUsers",
        "responses": {
          "200": {
            "description": "OK",
            "content": {
              "application/json": {
                "schema": {
                  "type": "array",
                  "items": {
                    "type": "object",
                    "properties": {
                      "id": {"type": "integer"},
                      "name": {"type": "string"}
                    }
                  }
                }
              }
            }
          }
        }
      },
      "post": {
        "operationId": "createUser",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "required": ["name"],
                "properties": {
                  "name": {"type": "string"}
                }
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": "Created",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "id": {"type": "integer"},
                    "name": {"type": "string"}
                  }
                }
              }
            }
          }
        }
      }
    },
    "/users/{id}": {
      "get": {
        "operationId": "getUser",
        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "integer"}}],
        "responses": {
          "200": {
            "description": "OK",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "id": {"type": "integer"},
                    "name": {"type": "string"}
                  }
                }
              }
            }
          }
        }
      }
    },
    "/xml-endpoint": {
      "get": {
        "responses": {
          "200": {
            "description": "OK",
            "content": {
              "application/xml": {
                "schema": {"type": "string"}
              }
            }
          }
        }
      }
    }
  }
}`

const testSchema2 = `{
  "openapi": "3.0.0",
  "info": {"title": "Orders API", "version": "1.0.0"},
  "paths": {
    "/orders": {
      "get": {
        "operationId": "listOrders",
        "responses": {
          "200": {
            "description": "OK",
            "content": {
              "application/json": {
                "schema": {
                  "type": "array",
                  "items": {
                    "type": "object",
                    "properties": {
                      "id": {"type": "integer"},
                      "total": {"type": "number"}
                    }
                  }
                }
              }
            }
          }
        }
      }
    }
  }
}`

func newTestHandler(t *testing.T) *MockHandler {
	t.Helper()
	v := cvt.NewValidator()
	err := v.RegisterSchema("test-api", []byte(testSchema))
	require.NoError(t, err)
	return NewMockHandler(v, []string{"test-api"}, HandlerConfig{Quiet: true})
}

func TestHandler_HappyPath(t *testing.T) {
	h := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

	var body interface{}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	assert.NoError(t, err, "response body should be valid JSON")
}

func TestHandler_ConcretePathParam(t *testing.T) {
	h := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

	var body map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	assert.NoError(t, err, "response body should be valid JSON")
}

func TestHandler_NotFound(t *testing.T) {
	h := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

	var body map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "no matching route", body["error"])
	assert.Equal(t, "GET", body["method"])
	assert.Equal(t, "/nonexistent", body["path"])
}

func TestHandler_NonJSONContentType(t *testing.T) {
	h := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/xml-endpoint", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotAcceptable, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

	var body map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "mock server only supports application/json responses", body["error"])
	assert.Equal(t, "GET", body["method"])
	assert.Equal(t, "/xml-endpoint", body["path"])
	assert.Contains(t, body["contentType"].(string), "xml")
}

func TestHandler_ValidateRequests_Valid(t *testing.T) {
	v := cvt.NewValidator()
	err := v.RegisterSchema("test-api", []byte(testSchema))
	require.NoError(t, err)

	h := NewMockHandler(v, []string{"test-api"}, HandlerConfig{
		ValidateRequests: true,
		Quiet:            true,
	})

	body := `{"name": "Alice"}`
	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
}

func TestHandler_ValidateRequests_Invalid(t *testing.T) {
	v := cvt.NewValidator()
	err := v.RegisterSchema("test-api", []byte(testSchema))
	require.NoError(t, err)

	h := NewMockHandler(v, []string{"test-api"}, HandlerConfig{
		ValidateRequests: true,
		Quiet:            true,
	})

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

	var body map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "request validation failed", body["error"])
	assert.NotNil(t, body["violations"])
}

func TestHandler_PanicRecovery(t *testing.T) {
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	h := newTestHandler(t)
	recovered := h.RecoverMiddleware(panicHandler)

	req := httptest.NewRequest(http.MethodGet, "/anything", nil)
	w := httptest.NewRecorder()

	recovered.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

	var body map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "internal server error", body["error"])
}

func TestHandler_MultiSchema(t *testing.T) {
	v := cvt.NewValidator()
	err := v.RegisterSchema("test-api", []byte(testSchema))
	require.NoError(t, err)
	err = v.RegisterSchema("orders-api", []byte(testSchema2))
	require.NoError(t, err)

	h := NewMockHandler(v, []string{"test-api", "orders-api"}, HandlerConfig{Quiet: true})

	// Test route from first schema
	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Test route from second schema
	req = httptest.NewRequest(http.MethodGet, "/orders", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Test unknown route returns 404
	req = httptest.NewRequest(http.MethodGet, "/unknown", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)

	var body map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "no matching route", body["error"])
}
