package cvt

import (
	"encoding/json"
	"strings"
	"testing"
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
		{"date", "2024"},
		{"date-time", "T"},
		{"uri", "https://"},
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
