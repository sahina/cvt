package cvtservice

import (
	"testing"

	"github.com/cvt/cvt/server/pb"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestDoc creates a basic OpenAPI document for testing.
func createTestDoc() *openapi3.T {
	return &openapi3.T{
		OpenAPI: "3.0.0",
		Info: &openapi3.Info{
			Title:   "Test API",
			Version: "1.0.0",
		},
		Paths: &openapi3.Paths{},
	}
}

// addPath adds a path with operations to the document.
func addPath(doc *openapi3.T, path string, methods ...string) {
	pathItem := &openapi3.PathItem{}
	for _, method := range methods {
		op := &openapi3.Operation{
			Responses: &openapi3.Responses{},
		}
		switch method {
		case "GET":
			pathItem.Get = op
		case "POST":
			pathItem.Post = op
		case "PUT":
			pathItem.Put = op
		case "DELETE":
			pathItem.Delete = op
		case "PATCH":
			pathItem.Patch = op
		}
	}
	doc.Paths.Set(path, pathItem)
}

func TestCompatibilityEngine_EndpointRemoved(t *testing.T) {
	engine := NewCompatibilityEngine()

	oldDoc := createTestDoc()
	addPath(oldDoc, "/pets", "GET", "POST")
	addPath(oldDoc, "/pets/{id}", "GET", "DELETE")

	newDoc := createTestDoc()
	addPath(newDoc, "/pets", "GET") // POST removed
	addPath(newDoc, "/pets/{id}", "GET", "DELETE")

	changes, compatible := engine.CompareSchemas(oldDoc, newDoc)

	assert.False(t, compatible)
	require.Len(t, changes, 1)
	assert.Equal(t, pb.BreakingChangeType_ENDPOINT_REMOVED, changes[0].Type)
	assert.Equal(t, "/pets", changes[0].Path)
	assert.Equal(t, "POST", changes[0].Method)
}

func TestCompatibilityEngine_MultipleEndpointsRemoved(t *testing.T) {
	engine := NewCompatibilityEngine()

	oldDoc := createTestDoc()
	addPath(oldDoc, "/pets", "GET", "POST", "PUT")
	addPath(oldDoc, "/users", "GET", "POST")

	newDoc := createTestDoc()
	addPath(newDoc, "/pets", "GET") // POST and PUT removed
	// /users completely removed

	changes, compatible := engine.CompareSchemas(oldDoc, newDoc)

	assert.False(t, compatible)
	// 2 from /pets (POST, PUT) + 2 from /users (GET, POST) = 4
	assert.Len(t, changes, 4)

	// All should be ENDPOINT_REMOVED
	for _, change := range changes {
		assert.Equal(t, pb.BreakingChangeType_ENDPOINT_REMOVED, change.Type)
	}
}

func TestCompatibilityEngine_RequiredParameterAdded(t *testing.T) {
	engine := NewCompatibilityEngine()

	oldDoc := createTestDoc()
	oldDoc.Paths.Set("/pets", &openapi3.PathItem{
		Get: &openapi3.Operation{
			Parameters: openapi3.Parameters{
				&openapi3.ParameterRef{
					Value: &openapi3.Parameter{
						Name:     "limit",
						In:       "query",
						Required: false,
					},
				},
			},
			Responses: &openapi3.Responses{},
		},
	})

	newDoc := createTestDoc()
	newDoc.Paths.Set("/pets", &openapi3.PathItem{
		Get: &openapi3.Operation{
			Parameters: openapi3.Parameters{
				&openapi3.ParameterRef{
					Value: &openapi3.Parameter{
						Name:     "limit",
						In:       "query",
						Required: false,
					},
				},
				&openapi3.ParameterRef{
					Value: &openapi3.Parameter{
						Name:     "status",
						In:       "query",
						Required: true, // New required parameter
					},
				},
			},
			Responses: &openapi3.Responses{},
		},
	})

	changes, compatible := engine.CompareSchemas(oldDoc, newDoc)

	assert.False(t, compatible)
	require.Len(t, changes, 1)
	assert.Equal(t, pb.BreakingChangeType_REQUIRED_PARAMETER_ADDED, changes[0].Type)
	assert.Contains(t, changes[0].Description, "status")
}

func TestCompatibilityEngine_RequiredFieldAdded(t *testing.T) {
	engine := NewCompatibilityEngine()

	oldDoc := createTestDoc()
	oldDoc.Paths.Set("/pets", &openapi3.PathItem{
		Post: &openapi3.Operation{
			RequestBody: &openapi3.RequestBodyRef{
				Value: &openapi3.RequestBody{
					Content: openapi3.Content{
						"application/json": &openapi3.MediaType{
							Schema: &openapi3.SchemaRef{
								Value: &openapi3.Schema{
									Type:     &openapi3.Types{"object"},
									Required: []string{"name"},
									Properties: openapi3.Schemas{
										"name": &openapi3.SchemaRef{
											Value: &openapi3.Schema{Type: &openapi3.Types{"string"}},
										},
									},
								},
							},
						},
					},
				},
			},
			Responses: &openapi3.Responses{},
		},
	})

	newDoc := createTestDoc()
	newDoc.Paths.Set("/pets", &openapi3.PathItem{
		Post: &openapi3.Operation{
			RequestBody: &openapi3.RequestBodyRef{
				Value: &openapi3.RequestBody{
					Content: openapi3.Content{
						"application/json": &openapi3.MediaType{
							Schema: &openapi3.SchemaRef{
								Value: &openapi3.Schema{
									Type:     &openapi3.Types{"object"},
									Required: []string{"name", "species"}, // species added as required
									Properties: openapi3.Schemas{
										"name": &openapi3.SchemaRef{
											Value: &openapi3.Schema{Type: &openapi3.Types{"string"}},
										},
										"species": &openapi3.SchemaRef{
											Value: &openapi3.Schema{Type: &openapi3.Types{"string"}},
										},
									},
								},
							},
						},
					},
				},
			},
			Responses: &openapi3.Responses{},
		},
	})

	changes, compatible := engine.CompareSchemas(oldDoc, newDoc)

	assert.False(t, compatible)
	require.Len(t, changes, 1)
	assert.Equal(t, pb.BreakingChangeType_REQUIRED_FIELD_ADDED, changes[0].Type)
	assert.Contains(t, changes[0].Description, "species")
}

func TestCompatibilityEngine_TypeChanged(t *testing.T) {
	engine := NewCompatibilityEngine()

	oldDoc := createTestDoc()
	oldDoc.Paths.Set("/pets", &openapi3.PathItem{
		Get: &openapi3.Operation{
			Parameters: openapi3.Parameters{
				&openapi3.ParameterRef{
					Value: &openapi3.Parameter{
						Name: "id",
						In:   "query",
						Schema: &openapi3.SchemaRef{
							Value: &openapi3.Schema{Type: &openapi3.Types{"string"}},
						},
					},
				},
			},
			Responses: &openapi3.Responses{},
		},
	})

	newDoc := createTestDoc()
	newDoc.Paths.Set("/pets", &openapi3.PathItem{
		Get: &openapi3.Operation{
			Parameters: openapi3.Parameters{
				&openapi3.ParameterRef{
					Value: &openapi3.Parameter{
						Name: "id",
						In:   "query",
						Schema: &openapi3.SchemaRef{
							Value: &openapi3.Schema{Type: &openapi3.Types{"integer"}}, // Type changed
						},
					},
				},
			},
			Responses: &openapi3.Responses{},
		},
	})

	changes, compatible := engine.CompareSchemas(oldDoc, newDoc)

	assert.False(t, compatible)
	require.Len(t, changes, 1)
	assert.Equal(t, pb.BreakingChangeType_TYPE_CHANGED, changes[0].Type)
	assert.Equal(t, "string", changes[0].OldValue)
	assert.Equal(t, "integer", changes[0].NewValue)
	assert.Contains(t, changes[0].Description, "id")
}

func TestCompatibilityEngine_EnumValueRemoved(t *testing.T) {
	engine := NewCompatibilityEngine()

	oldDoc := createTestDoc()
	oldDoc.Paths.Set("/pets", &openapi3.PathItem{
		Get: &openapi3.Operation{
			Parameters: openapi3.Parameters{
				&openapi3.ParameterRef{
					Value: &openapi3.Parameter{
						Name: "status",
						In:   "query",
						Schema: &openapi3.SchemaRef{
							Value: &openapi3.Schema{
								Type: &openapi3.Types{"string"},
								Enum: []interface{}{"available", "pending", "sold"},
							},
						},
					},
				},
			},
			Responses: &openapi3.Responses{},
		},
	})

	newDoc := createTestDoc()
	newDoc.Paths.Set("/pets", &openapi3.PathItem{
		Get: &openapi3.Operation{
			Parameters: openapi3.Parameters{
				&openapi3.ParameterRef{
					Value: &openapi3.Parameter{
						Name: "status",
						In:   "query",
						Schema: &openapi3.SchemaRef{
							Value: &openapi3.Schema{
								Type: &openapi3.Types{"string"},
								Enum: []interface{}{"available", "pending"}, // "sold" removed
							},
						},
					},
				},
			},
			Responses: &openapi3.Responses{},
		},
	})

	changes, compatible := engine.CompareSchemas(oldDoc, newDoc)

	assert.False(t, compatible)
	require.Len(t, changes, 1)
	assert.Equal(t, pb.BreakingChangeType_ENUM_VALUE_REMOVED, changes[0].Type)
	assert.Contains(t, changes[0].OldValue, "sold")
	assert.Contains(t, changes[0].Description, "status")
}

func TestCompatibilityEngine_ResponseFieldRemoved(t *testing.T) {
	engine := NewCompatibilityEngine()

	oldDoc := createTestDoc()
	oldDoc.Paths.Set("/pets", &openapi3.PathItem{
		Get: &openapi3.Operation{
			Responses: &openapi3.Responses{},
		},
	})
	oldDoc.Paths.Find("/pets").Get.Responses.Set("200", &openapi3.ResponseRef{
		Value: &openapi3.Response{
			Description: ptr("Success"),
			Content: openapi3.Content{
				"application/json": &openapi3.MediaType{
					Schema: &openapi3.SchemaRef{
						Value: &openapi3.Schema{
							Type: &openapi3.Types{"object"},
							Properties: openapi3.Schemas{
								"id":   &openapi3.SchemaRef{Value: &openapi3.Schema{Type: &openapi3.Types{"integer"}}},
								"name": &openapi3.SchemaRef{Value: &openapi3.Schema{Type: &openapi3.Types{"string"}}},
							},
						},
					},
				},
			},
		},
	})

	newDoc := createTestDoc()
	newDoc.Paths.Set("/pets", &openapi3.PathItem{
		Get: &openapi3.Operation{
			Responses: &openapi3.Responses{},
		},
	})
	newDoc.Paths.Find("/pets").Get.Responses.Set("200", &openapi3.ResponseRef{
		Value: &openapi3.Response{
			Description: ptr("Success"),
			Content: openapi3.Content{
				"application/json": &openapi3.MediaType{
					Schema: &openapi3.SchemaRef{
						Value: &openapi3.Schema{
							Type: &openapi3.Types{"object"},
							Properties: openapi3.Schemas{
								"id": &openapi3.SchemaRef{Value: &openapi3.Schema{Type: &openapi3.Types{"integer"}}},
								// "name" removed
							},
						},
					},
				},
			},
		},
	})

	changes, compatible := engine.CompareSchemas(oldDoc, newDoc)

	assert.False(t, compatible)
	require.Len(t, changes, 1)
	assert.Equal(t, pb.BreakingChangeType_RESPONSE_SCHEMA_CHANGED, changes[0].Type)
	assert.Contains(t, changes[0].Description, "name")
}

func TestCompatibilityEngine_NoBreakingChanges(t *testing.T) {
	engine := NewCompatibilityEngine()

	oldDoc := createTestDoc()
	addPath(oldDoc, "/pets", "GET", "POST")

	newDoc := createTestDoc()
	addPath(newDoc, "/pets", "GET", "POST")
	addPath(newDoc, "/users", "GET") // New endpoint (not breaking)

	changes, compatible := engine.CompareSchemas(oldDoc, newDoc)

	assert.True(t, compatible)
	assert.Len(t, changes, 0)
}

func TestCompatibilityEngine_CompatibleTypeChange(t *testing.T) {
	// int -> number is a widening conversion and should be allowed
	assert.True(t, isCompatibleTypeChange("integer", "number"))
	assert.True(t, isCompatibleTypeChange("int32", "int64"))
	assert.True(t, isCompatibleTypeChange("float", "double"))

	// Narrowing conversions should not be allowed
	assert.False(t, isCompatibleTypeChange("number", "integer"))
	assert.False(t, isCompatibleTypeChange("string", "integer"))
}

func TestCompatibilityEngine_NilPaths(t *testing.T) {
	engine := NewCompatibilityEngine()

	oldDoc := &openapi3.T{
		OpenAPI: "3.0.0",
		Info:    &openapi3.Info{Title: "Test", Version: "1.0"},
		Paths:   nil, // No paths
	}

	newDoc := createTestDoc()
	addPath(newDoc, "/pets", "GET")

	changes, compatible := engine.CompareSchemas(oldDoc, newDoc)

	assert.True(t, compatible)
	assert.Len(t, changes, 0)
}

// ptr is a helper function to create a pointer to a string.
func ptr(s string) *string {
	return &s
}
