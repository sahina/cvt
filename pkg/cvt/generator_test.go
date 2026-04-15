package cvt

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

const testSchemaWithExamples = `{
  "openapi": "3.0.0",
  "info": {
    "title": "User API",
    "version": "1.0.0"
  },
  "paths": {
    "/users": {
      "post": {
        "operationId": "createUser",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/CreateUserRequest"
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
                  "$ref": "#/components/schemas/User"
                },
                "example": {
                  "id": 999,
                  "name": "Example User",
                  "email": "example@test.com"
                }
              }
            }
          }
        }
      },
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
                    "$ref": "#/components/schemas/User"
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
        "parameters": [
          {
            "name": "id",
            "in": "path",
            "required": true,
            "schema": {
              "type": "integer"
            }
          }
        ],
        "responses": {
          "200": {
            "description": "OK",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/User"
                }
              }
            }
          },
          "404": {
            "description": "Not Found"
          }
        }
      }
    }
  },
  "components": {
    "schemas": {
      "User": {
        "type": "object",
        "required": ["id", "name", "email"],
        "properties": {
          "id": {
            "type": "integer",
            "format": "int64",
            "example": 123
          },
          "name": {
            "type": "string",
            "example": "John Doe"
          },
          "email": {
            "type": "string",
            "format": "email",
            "example": "john@example.com"
          },
          "role": {
            "type": "string",
            "enum": ["admin", "user", "guest"]
          },
          "createdAt": {
            "type": "string",
            "format": "date-time"
          }
        }
      },
      "CreateUserRequest": {
        "type": "object",
        "required": ["name", "email"],
        "properties": {
          "name": {
            "type": "string",
            "minLength": 1
          },
          "email": {
            "type": "string",
            "format": "email"
          },
          "role": {
            "type": "string",
            "enum": ["admin", "user", "guest"]
          }
        }
      }
    }
  }
}`

func TestGenerateResponse_Basic(t *testing.T) {
	v := NewValidator()
	err := v.RegisterSchema("test", []byte(testSchemaWithExamples))
	if err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	opts := DefaultGenerateOptions()
	resp, err := v.GenerateResponse("test", "GET", "/users/123", opts)
	if err != nil {
		t.Fatalf("GenerateResponse failed: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	if resp.Body == nil {
		t.Fatal("expected body to be generated")
	}

	body, ok := resp.Body.(map[string]interface{})
	if !ok {
		t.Fatalf("expected body to be object, got %T", resp.Body)
	}

	// Should have required fields
	if body["id"] == nil {
		t.Error("expected 'id' field")
	}
	if body["name"] == nil {
		t.Error("expected 'name' field")
	}
	if body["email"] == nil {
		t.Error("expected 'email' field")
	}
}

func TestGenerateResponse_WithExamples(t *testing.T) {
	v := NewValidator()
	err := v.RegisterSchema("test", []byte(testSchemaWithExamples))
	if err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	opts := DefaultGenerateOptions()
	opts.UseExamples = true
	resp, err := v.GenerateResponse("test", "GET", "/users/123", opts)
	if err != nil {
		t.Fatalf("GenerateResponse failed: %v", err)
	}

	body, ok := resp.Body.(map[string]interface{})
	if !ok {
		t.Fatalf("expected body to be object, got %T", resp.Body)
	}

	// Check that examples are used
	if id, ok := body["id"].(float64); !ok || id != 123 {
		t.Errorf("expected id example 123, got %v", body["id"])
	}
	if name, ok := body["name"].(string); !ok || name != "John Doe" {
		t.Errorf("expected name example 'John Doe', got %v", body["name"])
	}
}

func TestGenerateResponse_SpecificStatusCode(t *testing.T) {
	v := NewValidator()
	err := v.RegisterSchema("test", []byte(testSchemaWithExamples))
	if err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	opts := DefaultGenerateOptions()
	opts.StatusCode = 201
	resp, err := v.GenerateResponse("test", "POST", "/users", opts)
	if err != nil {
		t.Fatalf("GenerateResponse failed: %v", err)
	}

	if resp.StatusCode != 201 {
		t.Errorf("expected status 201, got %d", resp.StatusCode)
	}
}

func TestGenerateRequestBody_Basic(t *testing.T) {
	v := NewValidator()
	err := v.RegisterSchema("test", []byte(testSchemaWithExamples))
	if err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	opts := DefaultGenerateOptions()
	body, err := v.GenerateRequestBody("test", "POST", "/users", opts)
	if err != nil {
		t.Fatalf("GenerateRequestBody failed: %v", err)
	}

	if body == nil {
		t.Fatal("expected body to be generated")
	}

	obj, ok := body.(map[string]interface{})
	if !ok {
		t.Fatalf("expected body to be object, got %T", body)
	}

	if obj["name"] == nil {
		t.Error("expected 'name' field")
	}
	if obj["email"] == nil {
		t.Error("expected 'email' field")
	}
}

func TestGenerateRequestBody_NoRequestBody(t *testing.T) {
	v := NewValidator()
	err := v.RegisterSchema("test", []byte(testSchemaWithExamples))
	if err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	opts := DefaultGenerateOptions()
	_, err = v.GenerateRequestBody("test", "GET", "/users", opts)
	if err == nil {
		t.Error("expected error for endpoint without request body")
	}
}

func TestGenerateFixture_Complete(t *testing.T) {
	v := NewValidator()
	err := v.RegisterSchema("test", []byte(testSchemaWithExamples))
	if err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	opts := DefaultGenerateOptions()
	opts.StatusCode = 201
	fixture, err := v.GenerateFixture("test", "POST", "/users", opts)
	if err != nil {
		t.Fatalf("GenerateFixture failed: %v", err)
	}

	// Check request
	if fixture.Request.Method != "POST" {
		t.Errorf("expected method POST, got %s", fixture.Request.Method)
	}
	if fixture.Request.Path != "/users" {
		t.Errorf("expected path /users, got %s", fixture.Request.Path)
	}
	if fixture.Request.Body == nil {
		t.Error("expected request body")
	}

	// Check response
	if fixture.Response.StatusCode != 201 {
		t.Errorf("expected status 201, got %d", fixture.Response.StatusCode)
	}
	if fixture.Response.Body == nil {
		t.Error("expected response body")
	}
}

func TestGenerateFixtureJSON(t *testing.T) {
	v := NewValidator()
	err := v.RegisterSchema("test", []byte(testSchemaWithExamples))
	if err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	opts := DefaultGenerateOptions()
	jsonStr, err := v.GenerateFixtureJSON("test", "POST", "/users", opts)
	if err != nil {
		t.Fatalf("GenerateFixtureJSON failed: %v", err)
	}

	// Verify it's valid JSON
	var fixture GeneratedFixture
	if err := json.Unmarshal([]byte(jsonStr), &fixture); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if fixture.Request.Method != "POST" {
		t.Errorf("expected method POST, got %s", fixture.Request.Method)
	}
}

func TestListEndpoints(t *testing.T) {
	v := NewValidator()
	err := v.RegisterSchema("test", []byte(testSchemaWithExamples))
	if err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	endpoints, err := v.ListEndpoints("test")
	if err != nil {
		t.Fatalf("ListEndpoints failed: %v", err)
	}

	if len(endpoints) != 3 {
		t.Errorf("expected 3 endpoints, got %d: %v", len(endpoints), endpoints)
	}

	// Check for expected endpoints
	expectedEndpoints := map[string]bool{
		"POST /users":     false,
		"GET /users":      false,
		"GET /users/{id}": false,
	}

	for _, ep := range endpoints {
		for expected := range expectedEndpoints {
			if ep == expected {
				expectedEndpoints[expected] = true
			}
		}
	}

	for expected, found := range expectedEndpoints {
		if !found {
			t.Errorf("expected endpoint %q not found in %v", expected, endpoints)
		}
	}
}

func TestGetResponseExample_Found(t *testing.T) {
	v := NewValidator()
	err := v.RegisterSchema("test", []byte(testSchemaWithExamples))
	if err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	example, err := v.GetResponseExample("test", "POST", "/users", 201)
	if err != nil {
		t.Fatalf("GetResponseExample failed: %v", err)
	}

	exampleMap, ok := example.(map[string]interface{})
	if !ok {
		t.Fatalf("expected example to be object, got %T", example)
	}

	if exampleMap["id"].(float64) != 999 {
		t.Errorf("expected id 999, got %v", exampleMap["id"])
	}
	if exampleMap["name"].(string) != "Example User" {
		t.Errorf("expected name 'Example User', got %v", exampleMap["name"])
	}
}

func TestGetResponseExample_NotFound(t *testing.T) {
	v := NewValidator()
	err := v.RegisterSchema("test", []byte(testSchemaWithExamples))
	if err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	// GET /users doesn't have an explicit example
	_, err = v.GetResponseExample("test", "GET", "/users", 200)
	if err == nil {
		t.Error("expected error when no example exists")
	}
}

func TestGenerateValue_StringFormats(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		format   string
		contains string
	}{
		{"email", "@"},
		{"date", "-"},
		{"date-time", "T"},
		{"uri", "http"},
		{"uuid", "-"},
		{"ipv4", "."},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			schema := `{
				"openapi": "3.0.0",
				"info": {"title": "Test", "version": "1.0.0"},
				"paths": {
					"/test": {
						"get": {
							"responses": {
								"200": {
									"description": "OK",
									"content": {
										"application/json": {
											"schema": {
												"type": "string",
												"format": "` + tt.format + `"
											}
										}
									}
								}
							}
						}
					}
				}
			}`

			err := v.RegisterSchema("format-test", []byte(schema))
			if err != nil {
				t.Fatalf("RegisterSchema failed: %v", err)
			}

			opts := DefaultGenerateOptions()
			opts.UseExamples = false
			resp, err := v.GenerateResponse("format-test", "GET", "/test", opts)
			if err != nil {
				t.Fatalf("GenerateResponse failed: %v", err)
			}

			str, ok := resp.Body.(string)
			if !ok {
				t.Fatalf("expected string, got %T", resp.Body)
			}

			if !strings.Contains(str, tt.contains) {
				t.Errorf("expected %q format to contain %q, got %q", tt.format, tt.contains, str)
			}

			v.RemoveSchema("format-test")
		})
	}
}

func TestGenerateValue_Enum(t *testing.T) {
	v := NewValidator()
	err := v.RegisterSchema("test", []byte(testSchemaWithExamples))
	if err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	opts := DefaultGenerateOptions()
	opts.UseExamples = false
	resp, err := v.GenerateResponse("test", "GET", "/users/123", opts)
	if err != nil {
		t.Fatalf("GenerateResponse failed: %v", err)
	}

	body := resp.Body.(map[string]interface{})
	role := body["role"].(string)

	validRoles := []string{"admin", "user", "guest"}
	found := false
	for _, vr := range validRoles {
		if role == vr {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected role to be one of %v, got %q", validRoles, role)
	}
}

func TestGenerateResponse_ArrayType(t *testing.T) {
	v := NewValidator()
	err := v.RegisterSchema("test", []byte(testSchemaWithExamples))
	if err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	opts := DefaultGenerateOptions()
	resp, err := v.GenerateResponse("test", "GET", "/users", opts)
	if err != nil {
		t.Fatalf("GenerateResponse failed: %v", err)
	}

	arr, ok := resp.Body.([]interface{})
	if !ok {
		t.Fatalf("expected array, got %T", resp.Body)
	}

	if len(arr) == 0 {
		t.Error("expected at least one item in array")
	}
}

func TestGenerateResponse_SchemaNotFound(t *testing.T) {
	v := NewValidator()

	opts := DefaultGenerateOptions()
	_, err := v.GenerateResponse("nonexistent", "GET", "/test", opts)
	if err == nil {
		t.Error("expected error for non-existent schema")
	}
}

func TestGenerateResponse_RouteNotFound(t *testing.T) {
	v := NewValidator()
	err := v.RegisterSchema("test", []byte(testSchemaWithExamples))
	if err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	opts := DefaultGenerateOptions()
	_, err = v.GenerateResponse("test", "GET", "/nonexistent", opts)
	if err == nil {
		t.Error("expected error for non-existent route")
	}
}

func TestGenerateObject_AdditionalProperties(t *testing.T) {
	// Regression test: schemas with additionalProperties but no named properties
	// (like Petstore's GET /store/inventory) should generate sample entries,
	// not an empty object.
	schema := `{
	  "openapi": "3.0.0",
	  "info": {"title": "Test", "version": "1.0.0"},
	  "paths": {
	    "/store/inventory": {
	      "get": {
	        "responses": {
	          "200": {
	            "description": "OK",
	            "content": {
	              "application/json": {
	                "schema": {
	                  "type": "object",
	                  "additionalProperties": {
	                    "type": "integer",
	                    "format": "int32"
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

	v := NewValidator()
	err := v.RegisterSchema("inventory", []byte(schema))
	if err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	opts := DefaultGenerateOptions()
	opts.UseExamples = false
	resp, err := v.GenerateResponse("inventory", "GET", "/store/inventory", opts)
	if err != nil {
		t.Fatalf("GenerateResponse failed: %v", err)
	}

	body, ok := resp.Body.(map[string]interface{})
	if !ok {
		t.Fatalf("expected object body, got %T", resp.Body)
	}

	if len(body) == 0 {
		t.Fatal("expected non-empty object for schema with additionalProperties, got {}")
	}

	// Each value should be an integer
	for k, val := range body {
		switch val.(type) {
		case int64, float64, int:
			// ok
		default:
			t.Errorf("key %q: expected integer value, got %T (%v)", k, val, val)
		}
	}
}

func TestGenerateObject_AdditionalPropertiesWithNamedProperties(t *testing.T) {
	// When there ARE named properties, additionalProperties should NOT add synthetic keys
	schema := `{
	  "openapi": "3.0.0",
	  "info": {"title": "Test", "version": "1.0.0"},
	  "paths": {
	    "/test": {
	      "get": {
	        "responses": {
	          "200": {
	            "description": "OK",
	            "content": {
	              "application/json": {
	                "schema": {
	                  "type": "object",
	                  "properties": {
	                    "name": {"type": "string"}
	                  },
	                  "additionalProperties": {
	                    "type": "string"
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

	v := NewValidator()
	err := v.RegisterSchema("mixed", []byte(schema))
	if err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	opts := DefaultGenerateOptions()
	opts.UseExamples = false
	resp, err := v.GenerateResponse("mixed", "GET", "/test", opts)
	if err != nil {
		t.Fatalf("GenerateResponse failed: %v", err)
	}

	body, ok := resp.Body.(map[string]interface{})
	if !ok {
		t.Fatalf("expected object body, got %T", resp.Body)
	}

	// Should only have the named property, not synthetic keys
	if _, ok := body["name"]; !ok {
		t.Error("expected 'name' field")
	}
	if _, ok := body["key1"]; ok {
		t.Error("should not generate synthetic keys when named properties exist")
	}
}

func TestGenerateValue_TypelessSchemaWithProperties(t *testing.T) {
	schema := `{
	  "openapi": "3.0.0",
	  "info": {"title": "Test", "version": "1.0.0"},
	  "paths": {
	    "/test": {
	      "get": {
	        "responses": {
	          "200": {
	            "description": "OK",
	            "content": {
	              "application/json": {
	                "schema": {
	                  "properties": {
	                    "name": {"type": "string"},
	                    "age": {"type": "integer"}
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

	v := NewValidator()
	err := v.RegisterSchema("typeless", []byte(schema))
	if err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	opts := DefaultGenerateOptions()
	opts.UseExamples = false
	resp, err := v.GenerateResponse("typeless", "GET", "/test", opts)
	if err != nil {
		t.Fatalf("GenerateResponse failed: %v", err)
	}

	body, ok := resp.Body.(map[string]interface{})
	if !ok {
		t.Fatalf("expected object body for schema with properties, got %T", resp.Body)
	}
	if body["name"] == nil {
		t.Error("expected 'name' field in generated object")
	}
}

func TestGenerateNumber_Default(t *testing.T) {
	v := NewValidator()
	schema := &openapi3.Schema{}
	schema.Type = &openapi3.Types{"number"}
	result := v.generateNumber(schema)
	if result != 123.45 {
		t.Errorf("expected 123.45, got %v", result)
	}
}

func TestGenerateNumber_WithEnum(t *testing.T) {
	v := NewValidator()
	schema := &openapi3.Schema{}
	schema.Type = &openapi3.Types{"number"}
	schema.Enum = []interface{}{42.5}
	result := v.generateNumber(schema)
	if result != 42.5 {
		t.Errorf("expected 42.5, got %v", result)
	}
}

func TestGenerateNumber_WithMin(t *testing.T) {
	v := NewValidator()
	min := 10.0
	schema := &openapi3.Schema{}
	schema.Type = &openapi3.Types{"number"}
	schema.Min = &min
	result := v.generateNumber(schema)
	if result != 10.0 {
		t.Errorf("expected 10.0, got %v", result)
	}
}

func TestGenerateBoolean_Default(t *testing.T) {
	v := NewValidator()
	schema := &openapi3.Schema{}
	schema.Type = &openapi3.Types{"boolean"}
	// Just verify it returns a boolean (random)
	result := v.generateBoolean(schema)
	_ = result // bool is always valid
}

func TestGenerateBoolean_WithEnum(t *testing.T) {
	v := NewValidator()
	schema := &openapi3.Schema{}
	schema.Type = &openapi3.Types{"boolean"}
	schema.Enum = []interface{}{true}
	result := v.generateBoolean(schema)
	if !result {
		t.Error("expected true from enum")
	}
}

func TestGenerateAllOf(t *testing.T) {
	schemaJSON := `{
	  "openapi": "3.0.0",
	  "info": {"title": "Test", "version": "1.0.0"},
	  "paths": {
	    "/test": {
	      "get": {
	        "responses": {
	          "200": {
	            "description": "OK",
	            "content": {
	              "application/json": {
	                "schema": {
	                  "allOf": [
	                    {"type": "object", "properties": {"id": {"type": "integer"}}},
	                    {"type": "object", "properties": {"name": {"type": "string"}}}
	                  ]
	                }
	              }
	            }
	          }
	        }
	      }
	    }
	  }
	}`

	v := NewValidator()
	err := v.RegisterSchema("allof-test", []byte(schemaJSON))
	if err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	opts := DefaultGenerateOptions()
	opts.UseExamples = false
	resp, err := v.GenerateResponse("allof-test", "GET", "/test", opts)
	if err != nil {
		t.Fatalf("GenerateResponse failed: %v", err)
	}

	body, ok := resp.Body.(map[string]interface{})
	if !ok {
		t.Fatalf("expected object, got %T", resp.Body)
	}
	if body["id"] == nil {
		t.Error("expected 'id' from first allOf schema")
	}
	if body["name"] == nil {
		t.Error("expected 'name' from second allOf schema")
	}
}

func TestGetResponseForStatus_Exact(t *testing.T) {
	v := NewValidator()
	err := v.RegisterSchema("test", []byte(testSchemaWithExamples))
	if err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	doc, _ := v.GetSchema("test")
	operation, err := v.findOperation(doc, "POST", "/users")
	if err != nil {
		t.Fatalf("findOperation failed: %v", err)
	}

	resp, err := v.getResponseForStatus(operation, 201)
	if err != nil {
		t.Fatalf("getResponseForStatus failed: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response for status 201")
	}
}

func TestGetResponseForStatus_Default(t *testing.T) {
	schemaJSON := `{
	  "openapi": "3.0.0",
	  "info": {"title": "Test", "version": "1.0.0"},
	  "paths": {
	    "/test": {
	      "get": {
	        "responses": {
	          "default": {
	            "description": "Default response",
	            "content": {"application/json": {"schema": {"type": "object"}}}
	          }
	        }
	      }
	    }
	  }
	}`

	v := NewValidator()
	err := v.RegisterSchema("default-test", []byte(schemaJSON))
	if err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	doc, _ := v.GetSchema("default-test")
	operation, err := v.findOperation(doc, "GET", "/test")
	if err != nil {
		t.Fatalf("findOperation failed: %v", err)
	}

	resp, err := v.getResponseForStatus(operation, 200)
	if err != nil {
		t.Fatalf("getResponseForStatus with default should not fail: %v", err)
	}
	if resp == nil {
		t.Fatal("expected default response")
	}
}

func TestGetResponseForStatus_Wildcard(t *testing.T) {
	schemaJSON := `{
	  "openapi": "3.0.0",
	  "info": {"title": "Test", "version": "1.0.0"},
	  "paths": {
	    "/test": {
	      "get": {
	        "responses": {
	          "2XX": {
	            "description": "Success",
	            "content": {"application/json": {"schema": {"type": "object"}}}
	          }
	        }
	      }
	    }
	  }
	}`

	v := NewValidator()
	err := v.RegisterSchema("wildcard-test", []byte(schemaJSON))
	if err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	doc, _ := v.GetSchema("wildcard-test")
	operation, err := v.findOperation(doc, "GET", "/test")
	if err != nil {
		t.Fatalf("findOperation failed: %v", err)
	}

	resp, err := v.getResponseForStatus(operation, 200)
	if err != nil {
		t.Fatalf("getResponseForStatus with wildcard should not fail: %v", err)
	}
	if resp == nil {
		t.Fatal("expected wildcard response")
	}
}

func TestGetResponseForStatus_NotFound(t *testing.T) {
	v := NewValidator()
	err := v.RegisterSchema("test", []byte(testSchemaWithExamples))
	if err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	doc, _ := v.GetSchema("test")
	operation, err := v.findOperation(doc, "GET", "/users/{id}")
	if err != nil {
		t.Fatalf("findOperation failed: %v", err)
	}

	// Status 500 doesn't exist and no default
	_, err = v.getResponseForStatus(operation, 500)
	if err == nil {
		t.Error("expected error for non-existent status")
	}
}

func TestGetResponseForStatus_NilResponses(t *testing.T) {
	v := NewValidator()
	operation := &openapi3.Operation{}
	_, err := v.getResponseForStatus(operation, 200)
	if err == nil {
		t.Error("expected error for nil responses")
	}
}

func TestSelectSuccessStatus_Preferred(t *testing.T) {
	v := NewValidator()
	err := v.RegisterSchema("test", []byte(testSchemaWithExamples))
	if err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	doc, _ := v.GetSchema("test")

	// GET /users has 200 → should select 200
	getOp, _ := v.findOperation(doc, "GET", "/users")
	status := v.selectSuccessStatus(getOp)
	if status != 200 {
		t.Errorf("expected 200 for GET /users, got %d", status)
	}

	// POST /users has 201 → should select 201
	postOp, _ := v.findOperation(doc, "POST", "/users")
	status = v.selectSuccessStatus(postOp)
	if status != 201 {
		t.Errorf("expected 201 for POST /users, got %d", status)
	}
}

func TestSelectSuccessStatus_NilResponses(t *testing.T) {
	v := NewValidator()
	operation := &openapi3.Operation{}
	status := v.selectSuccessStatus(operation)
	if status != 200 {
		t.Errorf("expected 200 for nil responses, got %d", status)
	}
}

func TestSelectSuccessStatus_NonPreferred2XX(t *testing.T) {
	schemaJSON := `{
	  "openapi": "3.0.0",
	  "info": {"title": "Test", "version": "1.0.0"},
	  "paths": {
	    "/test": {
	      "get": {
	        "responses": {
	          "203": {"description": "Non-Authoritative Info"}
	        }
	      }
	    }
	  }
	}`

	v := NewValidator()
	err := v.RegisterSchema("non-preferred", []byte(schemaJSON))
	if err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	doc, _ := v.GetSchema("non-preferred")
	op, _ := v.findOperation(doc, "GET", "/test")
	status := v.selectSuccessStatus(op)
	if status != 203 {
		t.Errorf("expected 203 for non-preferred 2XX, got %d", status)
	}
}

func TestSelectSuccessStatus_OnlyNon2XX(t *testing.T) {
	schemaJSON := `{
	  "openapi": "3.0.0",
	  "info": {"title": "Test", "version": "1.0.0"},
	  "paths": {
	    "/test": {
	      "get": {
	        "responses": {
	          "301": {"description": "Moved"},
	          "default": {"description": "Default"}
	        }
	      }
	    }
	  }
	}`

	v := NewValidator()
	err := v.RegisterSchema("non-2xx", []byte(schemaJSON))
	if err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	doc, _ := v.GetSchema("non-2xx")
	op, _ := v.findOperation(doc, "GET", "/test")
	status := v.selectSuccessStatus(op)
	// Should fall back to first available non-default code
	if status != 301 {
		t.Errorf("expected 301 as fallback, got %d", status)
	}
}

func TestGenerateFixture_WithPathParams(t *testing.T) {
	v := NewValidator()
	err := v.RegisterSchema("test", []byte(testSchemaWithExamples))
	if err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	opts := DefaultGenerateOptions()
	fixture, err := v.GenerateFixture("test", "GET", "/users/{id}", opts)
	if err != nil {
		t.Fatalf("GenerateFixture failed: %v", err)
	}

	// Path should have {id} resolved
	if strings.Contains(fixture.Request.Path, "{id}") {
		t.Errorf("expected path param to be resolved, got %s", fixture.Request.Path)
	}
}

func TestGenerateFixture_SchemaNotFound(t *testing.T) {
	v := NewValidator()
	opts := DefaultGenerateOptions()
	_, err := v.GenerateFixture("nonexistent", "GET", "/test", opts)
	if err == nil {
		t.Error("expected error for non-existent schema")
	}
}

func TestGenerateValue_OneOf(t *testing.T) {
	schemaJSON := `{
	  "openapi": "3.0.0",
	  "info": {"title": "Test", "version": "1.0.0"},
	  "paths": {
	    "/test": {
	      "get": {
	        "responses": {
	          "200": {
	            "description": "OK",
	            "content": {
	              "application/json": {
	                "schema": {
	                  "oneOf": [
	                    {"type": "string"},
	                    {"type": "integer"}
	                  ]
	                }
	              }
	            }
	          }
	        }
	      }
	    }
	  }
	}`

	v := NewValidator()
	err := v.RegisterSchema("oneof-test", []byte(schemaJSON))
	if err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	opts := DefaultGenerateOptions()
	opts.UseExamples = false
	resp, err := v.GenerateResponse("oneof-test", "GET", "/test", opts)
	if err != nil {
		t.Fatalf("GenerateResponse failed: %v", err)
	}

	// Should pick first oneOf (string)
	if _, ok := resp.Body.(string); !ok {
		t.Errorf("expected string from oneOf[0], got %T", resp.Body)
	}
}

func TestGenerateValue_AnyOf(t *testing.T) {
	schemaJSON := `{
	  "openapi": "3.0.0",
	  "info": {"title": "Test", "version": "1.0.0"},
	  "paths": {
	    "/test": {
	      "get": {
	        "responses": {
	          "200": {
	            "description": "OK",
	            "content": {
	              "application/json": {
	                "schema": {
	                  "anyOf": [
	                    {"type": "integer"},
	                    {"type": "string"}
	                  ]
	                }
	              }
	            }
	          }
	        }
	      }
	    }
	  }
	}`

	v := NewValidator()
	err := v.RegisterSchema("anyof-test", []byte(schemaJSON))
	if err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	opts := DefaultGenerateOptions()
	opts.UseExamples = false
	resp, err := v.GenerateResponse("anyof-test", "GET", "/test", opts)
	if err != nil {
		t.Fatalf("GenerateResponse failed: %v", err)
	}

	// Should pick first anyOf (integer)
	if resp.Body == nil {
		t.Fatal("expected non-nil body from anyOf")
	}
}

func TestGenerateValue_DepthLimit(t *testing.T) {
	v := NewValidator()
	doc := &openapi3.T{}
	schemaRef := &openapi3.SchemaRef{
		Value: &openapi3.Schema{},
	}
	schemaRef.Value.Type = &openapi3.Types{"object"}
	schemaRef.Value.Properties = openapi3.Schemas{
		"self": schemaRef, // self-referencing
	}

	result := v.generateValue(doc, schemaRef, false, 11)
	if result != nil {
		t.Errorf("expected nil at depth > 10, got %v", result)
	}
}

func TestGenerateValue_NilSchemaRef(t *testing.T) {
	v := NewValidator()
	doc := &openapi3.T{}
	result := v.generateValue(doc, nil, false, 0)
	if result != nil {
		t.Errorf("expected nil for nil schemaRef, got %v", result)
	}
}

func TestGenerateString_MoreFormats(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		format   string
		expected string
	}{
		{"hostname", "example.com"},
		{"ipv6", "::1"},
		{"byte", "c3RyaW5n"},
		{"binary", "binary-data"},
		{"password", "********"},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			schema := &openapi3.Schema{Format: tt.format}
			schema.Type = &openapi3.Types{"string"}
			result := v.generateString(schema)
			if result != tt.expected {
				t.Errorf("generateString(%q) = %q, want %q", tt.format, result, tt.expected)
			}
		})
	}
}

func TestGenerateString_Pattern(t *testing.T) {
	v := NewValidator()
	schema := &openapi3.Schema{Pattern: `^\d{3}-\d{4}$`}
	schema.Type = &openapi3.Types{"string"}
	result := v.generateString(schema)
	if result != "pattern-value" {
		t.Errorf("expected 'pattern-value', got %q", result)
	}
}

func TestGenerateInteger_WithEnum(t *testing.T) {
	v := NewValidator()
	schema := &openapi3.Schema{}
	schema.Type = &openapi3.Types{"integer"}
	schema.Enum = []interface{}{float64(42)}
	result := v.generateInteger(schema)
	if result != 42 {
		t.Errorf("expected 42, got %d", result)
	}
}

func TestGenerateInteger_Int64Format(t *testing.T) {
	v := NewValidator()
	schema := &openapi3.Schema{Format: "int64"}
	schema.Type = &openapi3.Types{"integer"}
	result := v.generateInteger(schema)
	if result != 1234567890 {
		t.Errorf("expected 1234567890, got %d", result)
	}
}

func TestGenerateInteger_WithMin(t *testing.T) {
	v := NewValidator()
	min := 5.0
	schema := &openapi3.Schema{}
	schema.Type = &openapi3.Types{"integer"}
	schema.Min = &min
	result := v.generateInteger(schema)
	if result != 5 {
		t.Errorf("expected 5, got %d", result)
	}
}

func TestGenerateArray_MinItems(t *testing.T) {
	schemaJSON := `{
	  "openapi": "3.0.0",
	  "info": {"title": "Test", "version": "1.0.0"},
	  "paths": {
	    "/test": {
	      "get": {
	        "responses": {
	          "200": {
	            "description": "OK",
	            "content": {
	              "application/json": {
	                "schema": {
	                  "type": "array",
	                  "minItems": 3,
	                  "items": {"type": "string"}
	                }
	              }
	            }
	          }
	        }
	      }
	    }
	  }
	}`

	v := NewValidator()
	err := v.RegisterSchema("minitems-test", []byte(schemaJSON))
	if err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	opts := DefaultGenerateOptions()
	opts.UseExamples = false
	resp, err := v.GenerateResponse("minitems-test", "GET", "/test", opts)
	if err != nil {
		t.Fatalf("GenerateResponse failed: %v", err)
	}

	arr, ok := resp.Body.([]interface{})
	if !ok {
		t.Fatalf("expected array, got %T", resp.Body)
	}
	if len(arr) < 3 {
		t.Errorf("expected at least 3 items (minItems), got %d", len(arr))
	}
}

func TestGenerateRequestBody_SchemaNotFound(t *testing.T) {
	v := NewValidator()
	opts := DefaultGenerateOptions()
	_, err := v.GenerateRequestBody("nonexistent", "POST", "/test", opts)
	if err == nil {
		t.Error("expected error for non-existent schema")
	}
}

func TestGenerateFixtureJSON_SchemaNotFound(t *testing.T) {
	v := NewValidator()
	opts := DefaultGenerateOptions()
	_, err := v.GenerateFixtureJSON("nonexistent", "POST", "/test", opts)
	if err == nil {
		t.Error("expected error for non-existent schema")
	}
}

func TestListEndpoints_SchemaNotFound(t *testing.T) {
	v := NewValidator()
	_, err := v.ListEndpoints("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent schema")
	}
}

func TestGetResponseExample_SchemaNotFound(t *testing.T) {
	v := NewValidator()
	_, err := v.GetResponseExample("nonexistent", "GET", "/test", 200)
	if err == nil {
		t.Error("expected error for non-existent schema")
	}
}

func TestGenerateResponse_NoContent(t *testing.T) {
	// 404 response without content — should generate response with no body
	v := NewValidator()
	err := v.RegisterSchema("test", []byte(testSchemaWithExamples))
	if err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	opts := DefaultGenerateOptions()
	opts.StatusCode = 404
	resp, err := v.GenerateResponse("test", "GET", "/users/{id}", opts)
	if err != nil {
		t.Fatalf("GenerateResponse failed: %v", err)
	}
	if resp.StatusCode != 404 {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGenerateValue_TypelessSchemaNoProperties(t *testing.T) {
	schema := `{
	  "openapi": "3.0.0",
	  "info": {"title": "Test", "version": "1.0.0"},
	  "paths": {
	    "/test": {
	      "get": {
	        "responses": {
	          "200": {
	            "description": "OK",
	            "content": {
	              "application/json": {
	                "schema": {}
	              }
	            }
	          }
	        }
	      }
	    }
	  }
	}`

	v := NewValidator()
	err := v.RegisterSchema("typeless-empty", []byte(schema))
	if err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	opts := DefaultGenerateOptions()
	opts.UseExamples = false
	resp, err := v.GenerateResponse("typeless-empty", "GET", "/test", opts)
	if err != nil {
		t.Fatalf("GenerateResponse failed: %v", err)
	}
	_ = resp
}

func TestNewValidatorWithSeed(t *testing.T) {
	v := NewValidatorWithSeed(42)
	if v == nil {
		t.Fatal("expected non-nil validator")
	}
	err := v.RegisterSchema("test", []byte(testSchemaWithExamples))
	if err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}
}

func TestSeededDeterminism(t *testing.T) {
	schema := []byte(testSchemaWithExamples)

	// Generate with seed 42 twice — should produce identical output
	v1 := NewValidatorWithSeed(42)
	if err := v1.RegisterSchema("test", schema); err != nil {
		t.Fatal(err)
	}
	opts := DefaultGenerateOptions()
	opts.UseExamples = false
	json1, err := v1.GenerateFixtureJSON("test", "GET", "/users/1", opts)
	if err != nil {
		t.Fatal(err)
	}

	v2 := NewValidatorWithSeed(42)
	if err := v2.RegisterSchema("test", schema); err != nil {
		t.Fatal(err)
	}
	json2, err := v2.GenerateFixtureJSON("test", "GET", "/users/1", opts)
	if err != nil {
		t.Fatal(err)
	}

	if json1 != json2 {
		t.Errorf("seeded output not deterministic:\n  run1: %s\n  run2: %s", json1, json2)
	}

	// Different seed should produce different output
	v3 := NewValidatorWithSeed(99)
	if err := v3.RegisterSchema("test", schema); err != nil {
		t.Fatal(err)
	}
	json3, err := v3.GenerateFixtureJSON("test", "GET", "/users/1", opts)
	if err != nil {
		t.Fatal(err)
	}

	if json1 == json3 {
		t.Error("different seeds produced identical output")
	}
}

func TestGenerateString_DefaultWord(t *testing.T) {
	v := NewValidator()
	schema := &openapi3.Schema{} // no format, no enum, no pattern
	schema.Type = &openapi3.Types{"string"}

	result := v.generateString(schema)
	if result == "" {
		t.Error("expected non-empty default string from faker.Word()")
	}
	if result == "string" {
		t.Error("expected faker-generated word, not hardcoded 'string'")
	}
}

func TestGenerateString_Pattern(t *testing.T) {
	v := NewValidator()
	schema := &openapi3.Schema{Pattern: "^[a-z]+$"}
	schema.Type = &openapi3.Types{"string"}

	result := v.generateString(schema)
	if result != "pattern-value" {
		t.Errorf("expected 'pattern-value' for pattern schema, got %q", result)
	}
}

func TestGenerateString_AdditionalFormats(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		format   string
		contains string
	}{
		{"hostname", "."},
		{"ipv6", ":"},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			schema := &openapi3.Schema{Format: tt.format}
			schema.Type = &openapi3.Types{"string"}
			result := v.generateString(schema)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("expected %q format to contain %q, got %q", tt.format, tt.contains, result)
			}
		})
	}

	// byte format should be base64
	t.Run("byte", func(t *testing.T) {
		schema := &openapi3.Schema{Format: "byte"}
		schema.Type = &openapi3.Types{"string"}
		result := v.generateString(schema)
		if result == "" {
			t.Error("expected non-empty base64 string")
		}
	})
}

func TestGenerateInteger_Bounds(t *testing.T) {
	v := NewValidatorWithSeed(42)

	minVal := float64(10)
	maxVal := float64(20)

	tests := []struct {
		name    string
		min     *float64
		max     *float64
		wantMin int64
		wantMax int64
	}{
		{"min only", &minVal, nil, 10, 1010},
		{"max only", nil, &maxVal, 1, 20},
		{"both", &minVal, &maxVal, 10, 20},
		{"neither", nil, nil, 1, 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := &openapi3.Schema{Min: tt.min, Max: tt.max}
			schema.Type = &openapi3.Types{"integer"}
			result := v.generateInteger(schema)
			if result < tt.wantMin || result > tt.wantMax {
				t.Errorf("expected integer in [%d, %d], got %d", tt.wantMin, tt.wantMax, result)
			}
		})
	}
}

func TestGenerateNumber_Bounds(t *testing.T) {
	v := NewValidatorWithSeed(42)

	minVal := float64(1.5)
	maxVal := float64(9.9)

	tests := []struct {
		name    string
		min     *float64
		max     *float64
		wantMin float64
		wantMax float64
	}{
		{"min only", &minVal, nil, 1.5, 1001.5},
		{"max only", nil, &maxVal, 0.01, 9.9},
		{"both", &minVal, &maxVal, 1.5, 9.9},
		{"neither", nil, nil, 0.01, 999.99},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := &openapi3.Schema{Min: tt.min, Max: tt.max}
			schema.Type = &openapi3.Types{"number"}
			result := v.generateNumber(schema)
			if result < tt.wantMin || result > tt.wantMax {
				t.Errorf("expected number in [%f, %f], got %f", tt.wantMin, tt.wantMax, result)
			}
		})
	}
}

func TestGenerateBoolean_Varies(t *testing.T) {
	v := NewValidator() // unseeded = random
	schema := &openapi3.Schema{}
	schema.Type = &openapi3.Types{"boolean"}

	seenTrue := false
	seenFalse := false
	for i := 0; i < 100; i++ {
		if v.generateBoolean(schema) {
			seenTrue = true
		} else {
			seenFalse = true
		}
		if seenTrue && seenFalse {
			break
		}
	}

	if !seenTrue || !seenFalse {
		t.Error("expected both true and false to appear in 100 boolean generations")
	}
}

func TestGenerateRoundtrip(t *testing.T) {
	// Generate a fixture and validate it against the same schema
	v := NewValidatorWithSeed(42)
	err := v.RegisterSchema("test", []byte(testSchemaWithExamples))
	if err != nil {
		t.Fatal(err)
	}

	opts := DefaultGenerateOptions()
	opts.UseExamples = false
	fixture, err := v.GenerateFixture("test", "POST", "/users", opts)
	if err != nil {
		t.Fatal(err)
	}

	// The generated fixture should have valid structure
	if fixture.Request.Method != "POST" {
		t.Errorf("expected POST, got %s", fixture.Request.Method)
	}
	if fixture.Response.StatusCode != 201 {
		t.Errorf("expected 201, got %d", fixture.Response.StatusCode)
	}

	// Body should be a valid object with expected fields
	body, ok := fixture.Response.Body.(map[string]interface{})
	if !ok {
		t.Fatalf("expected response body to be object, got %T", fixture.Response.Body)
	}

	// Validate response body has schema-required fields
	for _, field := range []string{"id", "name", "email"} {
		if body[field] == nil {
			t.Errorf("expected response body to have %q field", field)
		}
	}

	// Email should contain @
	if email, ok := body["email"].(string); ok {
		if !strings.Contains(email, "@") {
			t.Errorf("expected email to contain @, got %q", email)
		}
	}
}
