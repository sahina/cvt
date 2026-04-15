package cvt

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// Helper to parse an inline OpenAPI v3 schema for compatibility tests.
func mustParseSchema(t *testing.T, content string) *openapi3.T {
	t.Helper()
	v := NewValidator()
	doc, err := v.parseSchema([]byte(content))
	if err != nil {
		t.Fatalf("failed to parse schema: %v", err)
	}
	return doc
}

const compatBaseSchema = `{
  "openapi": "3.0.0",
  "info": {"title": "Test API", "version": "1.0.0"},
  "paths": {
    "/users": {
      "get": {
        "operationId": "listUsers",
        "parameters": [
          {
            "name": "limit",
            "in": "query",
            "required": false,
            "schema": {"type": "integer", "enum": [10, 20, 50]}
          }
        ],
        "responses": {
          "200": {
            "description": "OK",
            "content": {
              "application/json": {
                "schema": {
                  "type": "array",
                  "items": {"type": "object", "properties": {"id": {"type": "integer"}, "name": {"type": "string"}}}
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
                  "name": {"type": "string"},
                  "email": {"type": "string"}
                }
              }
            }
          }
        },
        "responses": {
          "201": {"description": "Created"}
        }
      }
    },
    "/users/{id}": {
      "get": {
        "operationId": "getUser",
        "parameters": [
          {"name": "id", "in": "path", "required": true, "schema": {"type": "integer"}}
        ],
        "responses": {
          "200": {
            "description": "OK",
            "content": {
              "application/json": {
                "schema": {"type": "object", "properties": {"id": {"type": "integer"}, "name": {"type": "string"}}}
              }
            }
          }
        }
      },
      "delete": {
        "operationId": "deleteUser",
        "parameters": [
          {"name": "id", "in": "path", "required": true, "schema": {"type": "integer"}}
        ],
        "responses": {
          "204": {"description": "Deleted"}
        }
      }
    }
  }
}`

func TestNewCompatibilityEngine(t *testing.T) {
	e := NewCompatibilityEngine()
	if e == nil {
		t.Fatal("expected non-nil engine")
	}
}

func TestCompareSchemas_NoChanges(t *testing.T) {
	doc := mustParseSchema(t, compatBaseSchema)

	e := NewCompatibilityEngine()
	changes, compatible := e.CompareSchemas(doc, doc)

	if !compatible {
		t.Errorf("expected compatible, got %d breaking changes", len(changes))
		for _, c := range changes {
			t.Logf("  %s: %s", c.Type, c.Description)
		}
	}
}

func TestCompareSchemas_EndpointRemoved(t *testing.T) {
	oldDoc := mustParseSchema(t, compatBaseSchema)

	// New schema without DELETE /users/{id}
	newSchema := `{
	  "openapi": "3.0.0",
	  "info": {"title": "Test API", "version": "2.0.0"},
	  "paths": {
	    "/users": {
	      "get": {
	        "operationId": "listUsers",
	        "responses": {"200": {"description": "OK"}}
	      },
	      "post": {
	        "operationId": "createUser",
	        "requestBody": {
	          "required": true,
	          "content": {"application/json": {"schema": {"type": "object", "required": ["name"], "properties": {"name": {"type": "string"}}}}}
	        },
	        "responses": {"201": {"description": "Created"}}
	      }
	    },
	    "/users/{id}": {
	      "get": {
	        "operationId": "getUser",
	        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "integer"}}],
	        "responses": {"200": {"description": "OK"}}
	      }
	    }
	  }
	}`
	newDoc := mustParseSchema(t, newSchema)

	e := NewCompatibilityEngine()
	changes, compatible := e.CompareSchemas(oldDoc, newDoc)

	if compatible {
		t.Fatal("expected incompatible when endpoint removed")
	}

	found := false
	for _, c := range changes {
		if c.Type == BreakingChangeEndpointRemoved && c.Method == "DELETE" {
			found = true
		}
	}
	if !found {
		t.Error("expected ENDPOINT_REMOVED for DELETE /users/{id}")
	}
}

func TestCompareSchemas_RequiredParameterAdded(t *testing.T) {
	oldDoc := mustParseSchema(t, compatBaseSchema)

	// New schema adds required "sort" parameter to GET /users
	newSchema := `{
	  "openapi": "3.0.0",
	  "info": {"title": "Test API", "version": "2.0.0"},
	  "paths": {
	    "/users": {
	      "get": {
	        "operationId": "listUsers",
	        "parameters": [
	          {"name": "limit", "in": "query", "required": false, "schema": {"type": "integer", "enum": [10, 20, 50]}},
	          {"name": "sort", "in": "query", "required": true, "schema": {"type": "string"}}
	        ],
	        "responses": {"200": {"description": "OK"}}
	      },
	      "post": {
	        "operationId": "createUser",
	        "requestBody": {
	          "required": true,
	          "content": {"application/json": {"schema": {"type": "object", "required": ["name"], "properties": {"name": {"type": "string"}}}}}
	        },
	        "responses": {"201": {"description": "Created"}}
	      }
	    },
	    "/users/{id}": {
	      "get": {
	        "operationId": "getUser",
	        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "integer"}}],
	        "responses": {"200": {"description": "OK"}}
	      },
	      "delete": {
	        "operationId": "deleteUser",
	        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "integer"}}],
	        "responses": {"204": {"description": "Deleted"}}
	      }
	    }
	  }
	}`
	newDoc := mustParseSchema(t, newSchema)

	e := NewCompatibilityEngine()
	changes, compatible := e.CompareSchemas(oldDoc, newDoc)

	if compatible {
		t.Fatal("expected incompatible when required parameter added")
	}

	found := false
	for _, c := range changes {
		if c.Type == BreakingChangeRequiredParameterAdded {
			found = true
		}
	}
	if !found {
		t.Error("expected REQUIRED_PARAMETER_ADDED change")
	}
}

func TestCompareSchemas_RequiredFieldAdded(t *testing.T) {
	oldDoc := mustParseSchema(t, compatBaseSchema)

	// New schema adds required "email" field to POST /users request body
	newSchema := `{
	  "openapi": "3.0.0",
	  "info": {"title": "Test API", "version": "2.0.0"},
	  "paths": {
	    "/users": {
	      "get": {
	        "operationId": "listUsers",
	        "parameters": [{"name": "limit", "in": "query", "required": false, "schema": {"type": "integer", "enum": [10, 20, 50]}}],
	        "responses": {"200": {"description": "OK"}}
	      },
	      "post": {
	        "operationId": "createUser",
	        "requestBody": {
	          "required": true,
	          "content": {
	            "application/json": {
	              "schema": {
	                "type": "object",
	                "required": ["name", "email"],
	                "properties": {"name": {"type": "string"}, "email": {"type": "string"}}
	              }
	            }
	          }
	        },
	        "responses": {"201": {"description": "Created"}}
	      }
	    },
	    "/users/{id}": {
	      "get": {
	        "operationId": "getUser",
	        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "integer"}}],
	        "responses": {"200": {"description": "OK"}}
	      },
	      "delete": {
	        "operationId": "deleteUser",
	        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "integer"}}],
	        "responses": {"204": {"description": "Deleted"}}
	      }
	    }
	  }
	}`
	newDoc := mustParseSchema(t, newSchema)

	e := NewCompatibilityEngine()
	changes, compatible := e.CompareSchemas(oldDoc, newDoc)

	if compatible {
		t.Fatal("expected incompatible when required field added")
	}

	found := false
	for _, c := range changes {
		if c.Type == BreakingChangeRequiredFieldAdded {
			found = true
		}
	}
	if !found {
		t.Error("expected REQUIRED_FIELD_ADDED change")
	}
}

func TestCompareSchemas_TypeChanged(t *testing.T) {
	oldDoc := mustParseSchema(t, compatBaseSchema)

	// New schema changes "limit" parameter type from integer to string
	newSchema := `{
	  "openapi": "3.0.0",
	  "info": {"title": "Test API", "version": "2.0.0"},
	  "paths": {
	    "/users": {
	      "get": {
	        "operationId": "listUsers",
	        "parameters": [
	          {"name": "limit", "in": "query", "required": false, "schema": {"type": "string"}}
	        ],
	        "responses": {"200": {"description": "OK"}}
	      },
	      "post": {
	        "operationId": "createUser",
	        "requestBody": {
	          "required": true,
	          "content": {"application/json": {"schema": {"type": "object", "required": ["name"], "properties": {"name": {"type": "string"}}}}}
	        },
	        "responses": {"201": {"description": "Created"}}
	      }
	    },
	    "/users/{id}": {
	      "get": {
	        "operationId": "getUser",
	        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "integer"}}],
	        "responses": {"200": {"description": "OK"}}
	      },
	      "delete": {
	        "operationId": "deleteUser",
	        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "integer"}}],
	        "responses": {"204": {"description": "Deleted"}}
	      }
	    }
	  }
	}`
	newDoc := mustParseSchema(t, newSchema)

	e := NewCompatibilityEngine()
	changes, compatible := e.CompareSchemas(oldDoc, newDoc)

	if compatible {
		t.Fatal("expected incompatible when type changed")
	}

	found := false
	for _, c := range changes {
		if c.Type == BreakingChangeTypeChanged {
			found = true
		}
	}
	if !found {
		t.Error("expected TYPE_CHANGED change")
	}
}

func TestCompareSchemas_CompatibleTypeChange(t *testing.T) {
	// integer → number is a compatible widening
	oldSchema := `{
	  "openapi": "3.0.0",
	  "info": {"title": "Test", "version": "1.0.0"},
	  "paths": {
	    "/test": {
	      "get": {
	        "parameters": [{"name": "val", "in": "query", "schema": {"type": "integer"}}],
	        "responses": {"200": {"description": "OK"}}
	      }
	    }
	  }
	}`
	newSchema := `{
	  "openapi": "3.0.0",
	  "info": {"title": "Test", "version": "2.0.0"},
	  "paths": {
	    "/test": {
	      "get": {
	        "parameters": [{"name": "val", "in": "query", "schema": {"type": "number"}}],
	        "responses": {"200": {"description": "OK"}}
	      }
	    }
	  }
	}`
	oldDoc := mustParseSchema(t, oldSchema)
	newDoc := mustParseSchema(t, newSchema)

	e := NewCompatibilityEngine()
	changes, compatible := e.CompareSchemas(oldDoc, newDoc)

	if !compatible {
		t.Errorf("expected compatible for integer→number, got %d changes", len(changes))
		for _, c := range changes {
			t.Logf("  %s: %s", c.Type, c.Description)
		}
	}
}

func TestCompareSchemas_EnumValueRemoved(t *testing.T) {
	oldDoc := mustParseSchema(t, compatBaseSchema)

	// New schema removes "50" from limit enum
	newSchema := `{
	  "openapi": "3.0.0",
	  "info": {"title": "Test API", "version": "2.0.0"},
	  "paths": {
	    "/users": {
	      "get": {
	        "operationId": "listUsers",
	        "parameters": [
	          {"name": "limit", "in": "query", "required": false, "schema": {"type": "integer", "enum": [10, 20]}}
	        ],
	        "responses": {"200": {"description": "OK"}}
	      },
	      "post": {
	        "operationId": "createUser",
	        "requestBody": {
	          "required": true,
	          "content": {"application/json": {"schema": {"type": "object", "required": ["name"], "properties": {"name": {"type": "string"}}}}}
	        },
	        "responses": {"201": {"description": "Created"}}
	      }
	    },
	    "/users/{id}": {
	      "get": {
	        "operationId": "getUser",
	        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "integer"}}],
	        "responses": {"200": {"description": "OK"}}
	      },
	      "delete": {
	        "operationId": "deleteUser",
	        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "integer"}}],
	        "responses": {"204": {"description": "Deleted"}}
	      }
	    }
	  }
	}`
	newDoc := mustParseSchema(t, newSchema)

	e := NewCompatibilityEngine()
	changes, compatible := e.CompareSchemas(oldDoc, newDoc)

	if compatible {
		t.Fatal("expected incompatible when enum value removed")
	}

	found := false
	for _, c := range changes {
		if c.Type == BreakingChangeEnumValueRemoved {
			found = true
		}
	}
	if !found {
		t.Error("expected ENUM_VALUE_REMOVED change")
	}
}

func TestCompareSchemas_ResponseRemoved(t *testing.T) {
	oldDoc := mustParseSchema(t, compatBaseSchema)

	// New schema removes 200 response from GET /users/{id}
	newSchema := `{
	  "openapi": "3.0.0",
	  "info": {"title": "Test API", "version": "2.0.0"},
	  "paths": {
	    "/users": {
	      "get": {
	        "operationId": "listUsers",
	        "parameters": [{"name": "limit", "in": "query", "required": false, "schema": {"type": "integer", "enum": [10, 20, 50]}}],
	        "responses": {"200": {"description": "OK"}}
	      },
	      "post": {
	        "operationId": "createUser",
	        "requestBody": {
	          "required": true,
	          "content": {"application/json": {"schema": {"type": "object", "required": ["name"], "properties": {"name": {"type": "string"}}}}}
	        },
	        "responses": {"201": {"description": "Created"}}
	      }
	    },
	    "/users/{id}": {
	      "get": {
	        "operationId": "getUser",
	        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "integer"}}],
	        "responses": {
	          "404": {"description": "Not Found"}
	        }
	      },
	      "delete": {
	        "operationId": "deleteUser",
	        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "integer"}}],
	        "responses": {"204": {"description": "Deleted"}}
	      }
	    }
	  }
	}`
	newDoc := mustParseSchema(t, newSchema)

	e := NewCompatibilityEngine()
	changes, compatible := e.CompareSchemas(oldDoc, newDoc)

	if compatible {
		t.Fatal("expected incompatible when response removed")
	}

	found := false
	for _, c := range changes {
		if c.Type == BreakingChangeResponseSchemaChanged {
			found = true
		}
	}
	if !found {
		t.Error("expected RESPONSE_SCHEMA_CHANGED change")
	}
}

func TestCompareSchemas_NilPaths(t *testing.T) {
	e := NewCompatibilityEngine()

	// Both nil paths
	oldDoc := &openapi3.T{}
	newDoc := &openapi3.T{}

	changes, compatible := e.CompareSchemas(oldDoc, newDoc)
	if !compatible {
		t.Errorf("expected compatible with nil paths, got %d changes", len(changes))
	}
}

func TestCompareSchemas_OldNilNewHasPaths(t *testing.T) {
	e := NewCompatibilityEngine()

	oldDoc := &openapi3.T{}
	newDoc := mustParseSchema(t, compatBaseSchema)

	changes, compatible := e.CompareSchemas(oldDoc, newDoc)
	// Adding endpoints is not a breaking change
	if !compatible {
		t.Errorf("expected compatible when adding endpoints, got %d changes", len(changes))
	}
}

func TestCompareSchemas_MultipleBreakingChanges(t *testing.T) {
	oldDoc := mustParseSchema(t, compatBaseSchema)

	// New schema: remove DELETE, add required param, change type — multiple breaks
	newSchema := `{
	  "openapi": "3.0.0",
	  "info": {"title": "Test API", "version": "2.0.0"},
	  "paths": {
	    "/users": {
	      "get": {
	        "operationId": "listUsers",
	        "parameters": [
	          {"name": "limit", "in": "query", "required": false, "schema": {"type": "string"}},
	          {"name": "sort", "in": "query", "required": true, "schema": {"type": "string"}}
	        ],
	        "responses": {"200": {"description": "OK"}}
	      },
	      "post": {
	        "operationId": "createUser",
	        "requestBody": {
	          "required": true,
	          "content": {"application/json": {"schema": {"type": "object", "required": ["name"], "properties": {"name": {"type": "string"}}}}}
	        },
	        "responses": {"201": {"description": "Created"}}
	      }
	    },
	    "/users/{id}": {
	      "get": {
	        "operationId": "getUser",
	        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "integer"}}],
	        "responses": {"200": {"description": "OK"}}
	      }
	    }
	  }
	}`
	newDoc := mustParseSchema(t, newSchema)

	e := NewCompatibilityEngine()
	changes, compatible := e.CompareSchemas(oldDoc, newDoc)

	if compatible {
		t.Fatal("expected incompatible with multiple breaking changes")
	}

	if len(changes) < 2 {
		t.Errorf("expected multiple breaking changes, got %d", len(changes))
	}
}

func TestIsCompatibleTypeChange(t *testing.T) {
	tests := []struct {
		oldType    string
		newType    string
		compatible bool
	}{
		{"integer", "number", true},
		{"integer", "string", false},
		{"int32", "int64", true},
		{"int32", "integer", true},
		{"int32", "number", true},
		{"int64", "number", true},
		{"float", "double", true},
		{"float", "number", true},
		{"string", "integer", false},
		{"string", "string", false}, // same type, not in map
		{"boolean", "string", false},
	}

	for _, tt := range tests {
		t.Run(tt.oldType+"_to_"+tt.newType, func(t *testing.T) {
			result := isCompatibleTypeChange(tt.oldType, tt.newType)
			if result != tt.compatible {
				t.Errorf("isCompatibleTypeChange(%q, %q) = %v, want %v", tt.oldType, tt.newType, result, tt.compatible)
			}
		})
	}
}
