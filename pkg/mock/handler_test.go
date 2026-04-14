package mock

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
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

// testSchemaWithBasePath has a server URL with a path prefix, like Petstore's /api/v3.
// Routes should match WITHOUT the prefix (e.g., /items, not /api/v2/items).
const testSchemaWithBasePath = `{
  "openapi": "3.0.0",
  "info": {"title": "API with BasePath", "version": "1.0.0"},
  "servers": [
    {"url": "https://example.com/api/v2"}
  ],
  "paths": {
    "/items": {
      "get": {
        "operationId": "listItems",
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
      }
    },
    "/items/{id}": {
      "get": {
        "operationId": "getItem",
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

func TestSelectSuccessStatus(t *testing.T) {
	tests := []struct {
		name     string
		op       *openapi3.Operation
		expected int
	}{
		{
			name:     "nil responses returns 200",
			op:       &openapi3.Operation{Responses: nil},
			expected: 200,
		},
		{
			name: "only 201 defined returns 201",
			op: &openapi3.Operation{
				Responses: &openapi3.Responses{
					Extensions: map[string]interface{}{},
				},
			},
			expected: 201,
		},
		{
			name: "only 202 defined returns 202",
			op: &openapi3.Operation{
				Responses: &openapi3.Responses{
					Extensions: map[string]interface{}{},
				},
			},
			expected: 202,
		},
		{
			name: "only 204 defined returns 204",
			op: &openapi3.Operation{
				Responses: &openapi3.Responses{
					Extensions: map[string]interface{}{},
				},
			},
			expected: 204,
		},
		{
			name: "non-standard 2XX (206) returns 206",
			op: &openapi3.Operation{
				Responses: &openapi3.Responses{
					Extensions: map[string]interface{}{},
				},
			},
			expected: 206,
		},
		{
			name: "no 2XX codes returns 200",
			op: &openapi3.Operation{
				Responses: &openapi3.Responses{
					Extensions: map[string]interface{}{},
				},
			},
			expected: 200,
		},
	}

	// Set up responses for each test case
	// Case: only 201
	tests[1].op.Responses.Set("201", &openapi3.ResponseRef{Value: &openapi3.Response{Description: ptr("Created")}})
	// Case: only 202
	tests[2].op.Responses.Set("202", &openapi3.ResponseRef{Value: &openapi3.Response{Description: ptr("Accepted")}})
	// Case: only 204
	tests[3].op.Responses.Set("204", &openapi3.ResponseRef{Value: &openapi3.Response{Description: ptr("No Content")}})
	// Case: non-standard 206
	tests[4].op.Responses.Set("206", &openapi3.ResponseRef{Value: &openapi3.Response{Description: ptr("Partial Content")}})
	// Case: no 2XX codes (only 400)
	tests[5].op.Responses.Set("400", &openapi3.ResponseRef{Value: &openapi3.Response{Description: ptr("Bad Request")}})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectSuccessStatus(tt.op)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestHasJSONResponse(t *testing.T) {
	tests := []struct {
		name     string
		op       *openapi3.Operation
		status   int
		expected bool
	}{
		{
			name:     "nil responses returns false",
			op:       &openapi3.Operation{Responses: nil},
			status:   200,
			expected: false,
		},
		{
			name: "nil content returns false",
			op: func() *openapi3.Operation {
				op := &openapi3.Operation{Responses: &openapi3.Responses{Extensions: map[string]interface{}{}}}
				op.Responses.Set("200", &openapi3.ResponseRef{Value: &openapi3.Response{
					Description: ptr("OK"),
					Content:     nil,
				}})
				return op
			}(),
			status:   200,
			expected: false,
		},
		{
			name: "xml only returns false",
			op: func() *openapi3.Operation {
				op := &openapi3.Operation{Responses: &openapi3.Responses{Extensions: map[string]interface{}{}}}
				op.Responses.Set("200", &openapi3.ResponseRef{Value: &openapi3.Response{
					Description: ptr("OK"),
					Content: openapi3.Content{
						"application/xml": &openapi3.MediaType{},
					},
				}})
				return op
			}(),
			status:   200,
			expected: false,
		},
		{
			name: "json present returns true",
			op: func() *openapi3.Operation {
				op := &openapi3.Operation{Responses: &openapi3.Responses{Extensions: map[string]interface{}{}}}
				op.Responses.Set("200", &openapi3.ResponseRef{Value: &openapi3.Response{
					Description: ptr("OK"),
					Content: openapi3.Content{
						"application/json": &openapi3.MediaType{},
					},
				}})
				return op
			}(),
			status:   200,
			expected: true,
		},
		{
			name: "status not found returns false",
			op: func() *openapi3.Operation {
				op := &openapi3.Operation{Responses: &openapi3.Responses{Extensions: map[string]interface{}{}}}
				op.Responses.Set("201", &openapi3.ResponseRef{Value: &openapi3.Response{
					Description: ptr("Created"),
					Content: openapi3.Content{
						"application/json": &openapi3.MediaType{},
					},
				}})
				return op
			}(),
			status:   200,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasJSONResponse(tt.op, tt.status)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestGetFirstContentType(t *testing.T) {
	tests := []struct {
		name     string
		op       *openapi3.Operation
		status   int
		expected string
	}{
		{
			name:     "nil responses returns empty",
			op:       &openapi3.Operation{Responses: nil},
			status:   200,
			expected: "",
		},
		{
			name: "nil content returns empty",
			op: func() *openapi3.Operation {
				op := &openapi3.Operation{Responses: &openapi3.Responses{Extensions: map[string]interface{}{}}}
				op.Responses.Set("200", &openapi3.ResponseRef{Value: &openapi3.Response{
					Description: ptr("OK"),
					Content:     nil,
				}})
				return op
			}(),
			status:   200,
			expected: "",
		},
		{
			name: "returns first content type",
			op: func() *openapi3.Operation {
				op := &openapi3.Operation{Responses: &openapi3.Responses{Extensions: map[string]interface{}{}}}
				op.Responses.Set("200", &openapi3.ResponseRef{Value: &openapi3.Response{
					Description: ptr("OK"),
					Content: openapi3.Content{
						"application/xml": &openapi3.MediaType{},
					},
				}})
				return op
			}(),
			status:   200,
			expected: "application/xml",
		},
		{
			name: "status not found returns empty",
			op: func() *openapi3.Operation {
				op := &openapi3.Operation{Responses: &openapi3.Responses{Extensions: map[string]interface{}{}}}
				op.Responses.Set("201", &openapi3.ResponseRef{Value: &openapi3.Response{
					Description: ptr("Created"),
					Content: openapi3.Content{
						"application/json": &openapi3.MediaType{},
					},
				}})
				return op
			}(),
			status:   200,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getFirstContentType(tt.op, tt.status)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestLogRequest_NotQuiet(t *testing.T) {
	v := cvt.NewValidator()
	err := v.RegisterSchema("test-api", []byte(testSchema))
	require.NoError(t, err)

	h := NewMockHandler(v, []string{"test-api"}, HandlerConfig{Quiet: false})

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	// Should not panic when quiet=false
	h.logRequest(req, http.StatusOK, time.Now())
}

// ptr is a helper to create a pointer to a string.
func ptr(s string) *string { return &s }

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

func TestHandler_SchemaWithServerBasePath(t *testing.T) {
	v := cvt.NewValidator()
	err := v.RegisterSchema("basepath-api", []byte(testSchemaWithBasePath))
	require.NoError(t, err)

	h := NewMockHandler(v, []string{"basepath-api"}, HandlerConfig{Quiet: true})

	// Routes should match WITHOUT the /api/v2 prefix
	req := httptest.NewRequest(http.MethodGet, "/items", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, "GET /items should match (basePath stripped)")

	var body interface{}
	err = json.Unmarshal(w.Body.Bytes(), &body)
	assert.NoError(t, err, "response should be valid JSON")

	// Path params should work too
	req = httptest.NewRequest(http.MethodGet, "/items/42", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, "GET /items/42 should match (basePath stripped)")

	// The full prefixed path should NOT match (we strip it)
	req = httptest.NewRequest(http.MethodGet, "/api/v2/items", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code, "/api/v2/items should 404 (prefix is stripped)")
}

func TestHandler_MultiSchemaWithBasePath(t *testing.T) {
	v := cvt.NewValidator()
	// Schema 1: no server URL
	err := v.RegisterSchema("no-base", []byte(testSchema))
	require.NoError(t, err)
	// Schema 2: has server URL with /api/v2 prefix
	err = v.RegisterSchema("with-base", []byte(testSchemaWithBasePath))
	require.NoError(t, err)

	h := NewMockHandler(v, []string{"no-base", "with-base"}, HandlerConfig{Quiet: true})

	// /users from schema 1 (no base path)
	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, "GET /users should match schema 1")

	// /items from schema 2 (base path stripped)
	req = httptest.NewRequest(http.MethodGet, "/items", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, "GET /items should match schema 2 (basePath stripped)")
}

func TestStripServerBasePaths(t *testing.T) {
	t.Run("no servers returns same doc", func(t *testing.T) {
		doc := &openapi3.T{OpenAPI: "3.0.0"}
		result := stripServerBasePaths(doc)
		assert.Equal(t, doc, result, "should return same pointer when no servers")
	})

	t.Run("clears servers when present", func(t *testing.T) {
		doc := &openapi3.T{
			OpenAPI: "3.0.0",
			Servers: openapi3.Servers{
				{URL: "https://api.example.com/v1/api"},
			},
		}
		result := stripServerBasePaths(doc)
		assert.NotEqual(t, doc, result, "should return new doc")
		assert.Nil(t, result.Servers, "servers should be nil after stripping")
	})

	t.Run("does not mutate original doc", func(t *testing.T) {
		doc := &openapi3.T{
			OpenAPI: "3.0.0",
			Servers: openapi3.Servers{
				{URL: "https://api.example.com/v1"},
			},
		}
		_ = stripServerBasePaths(doc)
		assert.Len(t, doc.Servers, 1, "original doc should not be mutated")
		assert.Equal(t, "https://api.example.com/v1", doc.Servers[0].URL)
	})
}
