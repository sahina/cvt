package cvt

import (
	"testing"
)

const testSchema = `{
  "openapi": "3.0.0",
  "info": {
    "title": "Pet Store",
    "version": "1.0.0"
  },
  "paths": {
    "/pets": {
      "get": {
        "operationId": "listPets",
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
        "operationId": "createPet",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "required": ["name"],
                "properties": {
                  "name": {"type": "string"},
                  "tag": {"type": "string"}
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
    }
  }
}`

func TestNewValidator(t *testing.T) {
	v := NewValidator()
	if v == nil {
		t.Fatal("NewValidator returned nil")
	}
	if v.schemas == nil {
		t.Fatal("schemas map is nil")
	}
}

func TestRegisterSchema(t *testing.T) {
	v := NewValidator()

	err := v.RegisterSchema("test", []byte(testSchema))
	if err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	schemas := v.ListSchemas()
	if len(schemas) != 1 {
		t.Fatalf("expected 1 schema, got %d", len(schemas))
	}
	if schemas[0] != "test" {
		t.Fatalf("expected schema ID 'test', got %q", schemas[0])
	}
}

func TestRegisterSchema_EmptyID(t *testing.T) {
	v := NewValidator()

	err := v.RegisterSchema("", []byte(testSchema))
	if err == nil {
		t.Fatal("expected error for empty schema ID")
	}
}

func TestRegisterSchema_EmptyContent(t *testing.T) {
	v := NewValidator()

	err := v.RegisterSchema("test", []byte{})
	if err == nil {
		t.Fatal("expected error for empty content")
	}
}

func TestRegisterSchema_InvalidJSON(t *testing.T) {
	v := NewValidator()

	err := v.RegisterSchema("test", []byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestValidate_ValidRequest(t *testing.T) {
	v := NewValidator()

	err := v.RegisterSchema("test", []byte(testSchema))
	if err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	interaction := &Interaction{
		Method:       "GET",
		Path:         "/pets",
		StatusCode:   200,
		ResponseBody: `[{"id": 1, "name": "Fluffy"}]`,
		ResponseHeaders: map[string]string{
			"Content-Type": "application/json",
		},
	}

	result, err := v.Validate("test", interaction)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	if !result.Valid {
		t.Fatalf("expected valid, got errors: %v", result.Errors)
	}
}

func TestValidate_InvalidResponse(t *testing.T) {
	v := NewValidator()

	err := v.RegisterSchema("test", []byte(testSchema))
	if err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	interaction := &Interaction{
		Method:       "GET",
		Path:         "/pets",
		StatusCode:   200,
		ResponseBody: `{"invalid": "response"}`, // Should be an array
		ResponseHeaders: map[string]string{
			"Content-Type": "application/json",
		},
	}

	result, err := v.Validate("test", interaction)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	if result.Valid {
		t.Fatal("expected invalid result")
	}
	if len(result.Errors) == 0 {
		t.Fatal("expected errors")
	}
}

func TestValidate_RouteNotFound(t *testing.T) {
	v := NewValidator()

	err := v.RegisterSchema("test", []byte(testSchema))
	if err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	interaction := &Interaction{
		Method:       "GET",
		Path:         "/users", // Not in schema
		StatusCode:   200,
		ResponseBody: `[]`,
	}

	result, err := v.Validate("test", interaction)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	if result.Valid {
		t.Fatal("expected invalid result for non-existent route")
	}
}

func TestValidate_SchemaNotFound(t *testing.T) {
	v := NewValidator()

	interaction := &Interaction{
		Method:       "GET",
		Path:         "/pets",
		StatusCode:   200,
		ResponseBody: `[]`,
	}

	_, err := v.Validate("nonexistent", interaction)
	if err == nil {
		t.Fatal("expected error for non-existent schema")
	}
}

func TestRemoveSchema(t *testing.T) {
	v := NewValidator()

	err := v.RegisterSchema("test", []byte(testSchema))
	if err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	v.RemoveSchema("test")

	schemas := v.ListSchemas()
	if len(schemas) != 0 {
		t.Fatalf("expected 0 schemas after removal, got %d", len(schemas))
	}
}

func TestGetSchema(t *testing.T) {
	v := NewValidator()

	err := v.RegisterSchema("test", []byte(testSchema))
	if err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	doc, found := v.GetSchema("test")
	if !found {
		t.Fatal("expected to find schema")
	}
	if doc == nil {
		t.Fatal("expected non-nil document")
	}

	_, found = v.GetSchema("nonexistent")
	if found {
		t.Fatal("expected not to find nonexistent schema")
	}
}

func TestValidateRequest_Valid(t *testing.T) {
	v := NewValidator()
	err := v.RegisterSchema("test", []byte(testSchema))
	if err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	result, err := v.ValidateRequest("test", "GET", "/pets", nil, "")
	if err != nil {
		t.Fatalf("ValidateRequest failed: %v", err)
	}
	if !result.Valid {
		t.Errorf("expected valid request, got errors: %v", result.Errors)
	}
}

func TestValidateRequest_InvalidBody(t *testing.T) {
	v := NewValidator()
	err := v.RegisterSchema("test", []byte(testSchema))
	if err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	headers := map[string]string{"Content-Type": "application/json"}
	// POST /pets requires name field
	result, err := v.ValidateRequest("test", "POST", "/pets", headers, `{"invalid": true}`)
	if err != nil {
		t.Fatalf("ValidateRequest failed: %v", err)
	}
	if result.Valid {
		t.Error("expected invalid request for missing required fields")
	}
	if len(result.Errors) == 0 {
		t.Error("expected validation errors")
	}
}

func TestValidateRequest_RouteNotFound(t *testing.T) {
	v := NewValidator()
	err := v.RegisterSchema("test", []byte(testSchema))
	if err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	result, err := v.ValidateRequest("test", "GET", "/nonexistent", nil, "")
	if err != nil {
		t.Fatalf("ValidateRequest failed: %v", err)
	}
	if result.Valid {
		t.Error("expected invalid result for non-existent route")
	}
}

func TestValidateRequest_SchemaNotFound(t *testing.T) {
	v := NewValidator()
	_, err := v.ValidateRequest("nonexistent", "GET", "/test", nil, "")
	if err == nil {
		t.Error("expected error for non-existent schema")
	}
}
