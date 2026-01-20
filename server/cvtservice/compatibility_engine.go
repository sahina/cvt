// Package main provides compatibility checking functionality for the CVT server.
// This file implements detection of breaking changes between OpenAPI schema versions.
package cvtservice

import (
	"fmt"
	"strings"

	"github.com/sahina/cvt/server/pb"
	"github.com/getkin/kin-openapi/openapi3"
	"go.uber.org/zap"
)

// CompatibilityEngine checks for breaking changes between schema versions.
type CompatibilityEngine struct{}

// NewCompatibilityEngine creates a new CompatibilityEngine.
func NewCompatibilityEngine() *CompatibilityEngine {
	return &CompatibilityEngine{}
}

// CompareSchemas compares two OpenAPI documents and detects breaking changes.
// Returns a list of breaking changes and whether the schemas are compatible.
func (e *CompatibilityEngine) CompareSchemas(oldDoc, newDoc *openapi3.T) ([]*pb.BreakingChange, bool) {
	var changes []*pb.BreakingChange

	// Check for removed endpoints
	endpointChanges := e.checkRemovedEndpoints(oldDoc, newDoc)
	changes = append(changes, endpointChanges...)

	// Check for added required parameters
	paramChanges := e.checkAddedRequiredParameters(oldDoc, newDoc)
	changes = append(changes, paramChanges...)

	// Check for added required request body fields
	bodyChanges := e.checkAddedRequiredBodyFields(oldDoc, newDoc)
	changes = append(changes, bodyChanges...)

	// Check for type changes
	typeChanges := e.checkTypeChanges(oldDoc, newDoc)
	changes = append(changes, typeChanges...)

	// Check for removed enum values
	enumChanges := e.checkRemovedEnumValues(oldDoc, newDoc)
	changes = append(changes, enumChanges...)

	// Check for response schema changes
	responseChanges := e.checkResponseChanges(oldDoc, newDoc)
	changes = append(changes, responseChanges...)

	compatible := len(changes) == 0
	return changes, compatible
}

// checkRemovedEndpoints detects endpoints that existed in old schema but not in new.
func (e *CompatibilityEngine) checkRemovedEndpoints(oldDoc, newDoc *openapi3.T) []*pb.BreakingChange {
	var changes []*pb.BreakingChange

	if oldDoc.Paths == nil {
		return changes
	}

	for path, oldPathItem := range oldDoc.Paths.Map() {
		var newPathItem *openapi3.PathItem
		if newDoc.Paths != nil {
			newPathItem = newDoc.Paths.Find(path)
		}

		// Check each HTTP method
		methods := []struct {
			name     string
			oldOp    *openapi3.Operation
			newOp    *openapi3.Operation
			getNewOp func(*openapi3.PathItem) *openapi3.Operation
		}{
			{"GET", oldPathItem.Get, nil, func(p *openapi3.PathItem) *openapi3.Operation { return p.Get }},
			{"POST", oldPathItem.Post, nil, func(p *openapi3.PathItem) *openapi3.Operation { return p.Post }},
			{"PUT", oldPathItem.Put, nil, func(p *openapi3.PathItem) *openapi3.Operation { return p.Put }},
			{"DELETE", oldPathItem.Delete, nil, func(p *openapi3.PathItem) *openapi3.Operation { return p.Delete }},
			{"PATCH", oldPathItem.Patch, nil, func(p *openapi3.PathItem) *openapi3.Operation { return p.Patch }},
			{"HEAD", oldPathItem.Head, nil, func(p *openapi3.PathItem) *openapi3.Operation { return p.Head }},
			{"OPTIONS", oldPathItem.Options, nil, func(p *openapi3.PathItem) *openapi3.Operation { return p.Options }},
		}

		for _, m := range methods {
			if m.oldOp == nil {
				continue
			}

			// Check if endpoint exists in new schema
			exists := false
			if newPathItem != nil {
				if op := m.getNewOp(newPathItem); op != nil {
					exists = true
				}
			}

			if !exists {
				changes = append(changes, &pb.BreakingChange{
					Type:        pb.BreakingChangeType_ENDPOINT_REMOVED,
					Path:        path,
					Method:      m.name,
					Description: fmt.Sprintf("Endpoint %s %s was removed", m.name, path),
				})
				Info("Detected breaking change: endpoint removed",
					zap.String("method", m.name),
					zap.String("path", path))
			}
		}
	}

	return changes
}

// checkAddedRequiredParameters detects new required parameters added to existing endpoints.
func (e *CompatibilityEngine) checkAddedRequiredParameters(oldDoc, newDoc *openapi3.T) []*pb.BreakingChange {
	var changes []*pb.BreakingChange

	if newDoc.Paths == nil {
		return changes
	}

	for path, newPathItem := range newDoc.Paths.Map() {
		var oldPathItem *openapi3.PathItem
		if oldDoc.Paths != nil {
			oldPathItem = oldDoc.Paths.Find(path)
		}
		if oldPathItem == nil {
			continue // New endpoint, not a breaking change
		}

		// Check each method's parameters
		methodChecks := []struct {
			name  string
			oldOp *openapi3.Operation
			newOp *openapi3.Operation
		}{
			{"GET", oldPathItem.Get, newPathItem.Get},
			{"POST", oldPathItem.Post, newPathItem.Post},
			{"PUT", oldPathItem.Put, newPathItem.Put},
			{"DELETE", oldPathItem.Delete, newPathItem.Delete},
			{"PATCH", oldPathItem.Patch, newPathItem.Patch},
		}

		for _, m := range methodChecks {
			if m.oldOp == nil || m.newOp == nil {
				continue
			}

			// Build map of old parameter names
			oldParams := make(map[string]bool)
			for _, param := range m.oldOp.Parameters {
				if param.Value != nil {
					oldParams[param.Value.Name] = true
				}
			}

			// Check for new required parameters
			for _, param := range m.newOp.Parameters {
				if param.Value == nil {
					continue
				}
				if param.Value.Required && !oldParams[param.Value.Name] {
					changes = append(changes, &pb.BreakingChange{
						Type:        pb.BreakingChangeType_REQUIRED_PARAMETER_ADDED,
						Path:        path,
						Method:      m.name,
						Description: fmt.Sprintf("Required parameter '%s' was added to %s %s", param.Value.Name, m.name, path),
					})
					Info("Detected breaking change: required parameter added",
						zap.String("method", m.name),
						zap.String("path", path),
						zap.String("parameter", param.Value.Name))
				}
			}
		}
	}

	return changes
}

// checkAddedRequiredBodyFields detects new required fields in request bodies.
func (e *CompatibilityEngine) checkAddedRequiredBodyFields(oldDoc, newDoc *openapi3.T) []*pb.BreakingChange {
	var changes []*pb.BreakingChange

	if newDoc.Paths == nil {
		return changes
	}

	for path, newPathItem := range newDoc.Paths.Map() {
		var oldPathItem *openapi3.PathItem
		if oldDoc.Paths != nil {
			oldPathItem = oldDoc.Paths.Find(path)
		}
		if oldPathItem == nil {
			continue
		}

		// Check methods with request bodies
		methodChecks := []struct {
			name  string
			oldOp *openapi3.Operation
			newOp *openapi3.Operation
		}{
			{"POST", oldPathItem.Post, newPathItem.Post},
			{"PUT", oldPathItem.Put, newPathItem.Put},
			{"PATCH", oldPathItem.Patch, newPathItem.Patch},
		}

		for _, m := range methodChecks {
			if m.oldOp == nil || m.newOp == nil {
				continue
			}

			bodyChanges := e.compareRequestBodies(m.oldOp.RequestBody, m.newOp.RequestBody, path, m.name)
			changes = append(changes, bodyChanges...)
		}
	}

	return changes
}

// compareRequestBodies compares two request bodies for added required fields.
func (e *CompatibilityEngine) compareRequestBodies(oldBody, newBody *openapi3.RequestBodyRef, path, method string) []*pb.BreakingChange {
	var changes []*pb.BreakingChange

	if newBody == nil || newBody.Value == nil {
		return changes
	}

	newContent := newBody.Value.Content
	if newContent == nil {
		return changes
	}

	// Get old required fields
	oldRequired := make(map[string]bool)
	if oldBody != nil && oldBody.Value != nil && oldBody.Value.Content != nil {
		for _, mediaType := range oldBody.Value.Content {
			if mediaType.Schema != nil && mediaType.Schema.Value != nil {
				for _, field := range mediaType.Schema.Value.Required {
					oldRequired[field] = true
				}
			}
		}
	}

	// Check new required fields
	for _, mediaType := range newContent {
		if mediaType.Schema == nil || mediaType.Schema.Value == nil {
			continue
		}

		for _, field := range mediaType.Schema.Value.Required {
			if !oldRequired[field] {
				changes = append(changes, &pb.BreakingChange{
					Type:        pb.BreakingChangeType_REQUIRED_FIELD_ADDED,
					Path:        path,
					Method:      method,
					Description: fmt.Sprintf("Required field '%s' was added to request body of %s %s", field, method, path),
				})
				Info("Detected breaking change: required field added",
					zap.String("method", method),
					zap.String("path", path),
					zap.String("field", field))
			}
		}
	}

	return changes
}

// checkTypeChanges detects type changes in parameters or request body fields.
func (e *CompatibilityEngine) checkTypeChanges(oldDoc, newDoc *openapi3.T) []*pb.BreakingChange {
	var changes []*pb.BreakingChange

	if oldDoc.Paths == nil || newDoc.Paths == nil {
		return changes
	}

	for path, newPathItem := range newDoc.Paths.Map() {
		oldPathItem := oldDoc.Paths.Find(path)
		if oldPathItem == nil {
			continue
		}

		// Check each method
		methodChecks := []struct {
			name  string
			oldOp *openapi3.Operation
			newOp *openapi3.Operation
		}{
			{"GET", oldPathItem.Get, newPathItem.Get},
			{"POST", oldPathItem.Post, newPathItem.Post},
			{"PUT", oldPathItem.Put, newPathItem.Put},
			{"DELETE", oldPathItem.Delete, newPathItem.Delete},
			{"PATCH", oldPathItem.Patch, newPathItem.Patch},
		}

		for _, m := range methodChecks {
			if m.oldOp == nil || m.newOp == nil {
				continue
			}

			// Check parameter type changes
			paramChanges := e.checkParameterTypeChanges(m.oldOp, m.newOp, path, m.name)
			changes = append(changes, paramChanges...)

			// Check request body type changes
			bodyChanges := e.checkRequestBodyTypeChanges(m.oldOp.RequestBody, m.newOp.RequestBody, path, m.name)
			changes = append(changes, bodyChanges...)
		}
	}

	return changes
}

// checkParameterTypeChanges compares parameter types between old and new operations.
func (e *CompatibilityEngine) checkParameterTypeChanges(oldOp, newOp *openapi3.Operation, path, method string) []*pb.BreakingChange {
	var changes []*pb.BreakingChange

	// Build map of old parameters with their types
	oldParams := make(map[string]string)
	for _, param := range oldOp.Parameters {
		if param.Value != nil && param.Value.Schema != nil && param.Value.Schema.Value != nil {
			if len(param.Value.Schema.Value.Type.Slice()) > 0 {
				oldParams[param.Value.Name] = param.Value.Schema.Value.Type.Slice()[0]
			}
		}
	}

	// Check for type changes
	for _, param := range newOp.Parameters {
		if param.Value == nil || param.Value.Schema == nil || param.Value.Schema.Value == nil {
			continue
		}

		oldType, exists := oldParams[param.Value.Name]
		if !exists {
			continue // New parameter, handled by checkAddedRequiredParameters
		}

		newType := ""
		if len(param.Value.Schema.Value.Type.Slice()) > 0 {
			newType = param.Value.Schema.Value.Type.Slice()[0]
		}

		if oldType != newType && !isCompatibleTypeChange(oldType, newType) {
			changes = append(changes, &pb.BreakingChange{
				Type:        pb.BreakingChangeType_TYPE_CHANGED,
				Path:        path,
				Method:      method,
				OldValue:    oldType,
				NewValue:    newType,
				Description: fmt.Sprintf("Type of parameter '%s' changed from '%s' to '%s' in %s %s", param.Value.Name, oldType, newType, method, path),
			})
			Info("Detected breaking change: type changed",
				zap.String("method", method),
				zap.String("path", path),
				zap.String("field", param.Value.Name),
				zap.String("oldType", oldType),
				zap.String("newType", newType))
		}
	}

	return changes
}

// checkRequestBodyTypeChanges compares request body property types.
func (e *CompatibilityEngine) checkRequestBodyTypeChanges(oldBody, newBody *openapi3.RequestBodyRef, path, method string) []*pb.BreakingChange {
	var changes []*pb.BreakingChange

	if oldBody == nil || newBody == nil {
		return changes
	}
	if oldBody.Value == nil || newBody.Value == nil {
		return changes
	}

	// Get old properties and types
	oldProps := make(map[string]string)
	if oldBody.Value.Content != nil {
		for _, mediaType := range oldBody.Value.Content {
			if mediaType.Schema != nil && mediaType.Schema.Value != nil {
				for propName, prop := range mediaType.Schema.Value.Properties {
					if prop.Value != nil && len(prop.Value.Type.Slice()) > 0 {
						oldProps[propName] = prop.Value.Type.Slice()[0]
					}
				}
			}
		}
	}

	// Check new properties for type changes
	if newBody.Value.Content != nil {
		for _, mediaType := range newBody.Value.Content {
			if mediaType.Schema == nil || mediaType.Schema.Value == nil {
				continue
			}

			for propName, prop := range mediaType.Schema.Value.Properties {
				if prop.Value == nil || len(prop.Value.Type.Slice()) == 0 {
					continue
				}

				oldType, exists := oldProps[propName]
				if !exists {
					continue // New property
				}

				newType := prop.Value.Type.Slice()[0]
				if oldType != newType && !isCompatibleTypeChange(oldType, newType) {
					changes = append(changes, &pb.BreakingChange{
						Type:        pb.BreakingChangeType_TYPE_CHANGED,
						Path:        path,
						Method:      method,
						OldValue:    oldType,
						NewValue:    newType,
						Description: fmt.Sprintf("Type of field '%s' in request body changed from '%s' to '%s' in %s %s", propName, oldType, newType, method, path),
					})
				}
			}
		}
	}

	return changes
}

// checkRemovedEnumValues detects enum values that were removed.
func (e *CompatibilityEngine) checkRemovedEnumValues(oldDoc, newDoc *openapi3.T) []*pb.BreakingChange {
	var changes []*pb.BreakingChange

	if oldDoc.Paths == nil || newDoc.Paths == nil {
		return changes
	}

	for path, newPathItem := range newDoc.Paths.Map() {
		oldPathItem := oldDoc.Paths.Find(path)
		if oldPathItem == nil {
			continue
		}

		// Check each method
		methodChecks := []struct {
			name  string
			oldOp *openapi3.Operation
			newOp *openapi3.Operation
		}{
			{"GET", oldPathItem.Get, newPathItem.Get},
			{"POST", oldPathItem.Post, newPathItem.Post},
			{"PUT", oldPathItem.Put, newPathItem.Put},
			{"DELETE", oldPathItem.Delete, newPathItem.Delete},
			{"PATCH", oldPathItem.Patch, newPathItem.Patch},
		}

		for _, m := range methodChecks {
			if m.oldOp == nil || m.newOp == nil {
				continue
			}

			// Check parameter enums
			enumChanges := e.checkParameterEnumChanges(m.oldOp, m.newOp, path, m.name)
			changes = append(changes, enumChanges...)
		}
	}

	return changes
}

// checkParameterEnumChanges compares enum values in parameters.
func (e *CompatibilityEngine) checkParameterEnumChanges(oldOp, newOp *openapi3.Operation, path, method string) []*pb.BreakingChange {
	var changes []*pb.BreakingChange

	// Build map of old parameter enums
	oldEnums := make(map[string]map[string]bool)
	for _, param := range oldOp.Parameters {
		if param.Value != nil && param.Value.Schema != nil && param.Value.Schema.Value != nil {
			if len(param.Value.Schema.Value.Enum) > 0 {
				enumSet := make(map[string]bool)
				for _, val := range param.Value.Schema.Value.Enum {
					enumSet[fmt.Sprint(val)] = true
				}
				oldEnums[param.Value.Name] = enumSet
			}
		}
	}

	// Check for removed enum values
	for _, param := range newOp.Parameters {
		if param.Value == nil || param.Value.Schema == nil || param.Value.Schema.Value == nil {
			continue
		}

		oldEnumSet, exists := oldEnums[param.Value.Name]
		if !exists || len(param.Value.Schema.Value.Enum) == 0 {
			continue
		}

		// Check which old values are missing
		newEnumSet := make(map[string]bool)
		for _, val := range param.Value.Schema.Value.Enum {
			newEnumSet[fmt.Sprint(val)] = true
		}

		var removedValues []string
		for oldVal := range oldEnumSet {
			if !newEnumSet[oldVal] {
				removedValues = append(removedValues, oldVal)
			}
		}

		if len(removedValues) > 0 {
			changes = append(changes, &pb.BreakingChange{
				Type:        pb.BreakingChangeType_ENUM_VALUE_REMOVED,
				Path:        path,
				Method:      method,
				OldValue:    strings.Join(removedValues, ", "),
				Description: fmt.Sprintf("Enum values [%s] were removed from parameter '%s' in %s %s", strings.Join(removedValues, ", "), param.Value.Name, method, path),
			})
			Info("Detected breaking change: enum values removed",
				zap.String("method", method),
				zap.String("path", path),
				zap.String("field", param.Value.Name),
				zap.Strings("removedValues", removedValues))
		}
	}

	return changes
}

// checkResponseChanges detects breaking changes in response schemas.
func (e *CompatibilityEngine) checkResponseChanges(oldDoc, newDoc *openapi3.T) []*pb.BreakingChange {
	var changes []*pb.BreakingChange

	if oldDoc.Paths == nil || newDoc.Paths == nil {
		return changes
	}

	for path, newPathItem := range newDoc.Paths.Map() {
		oldPathItem := oldDoc.Paths.Find(path)
		if oldPathItem == nil {
			continue
		}

		// Check each method
		methodChecks := []struct {
			name  string
			oldOp *openapi3.Operation
			newOp *openapi3.Operation
		}{
			{"GET", oldPathItem.Get, newPathItem.Get},
			{"POST", oldPathItem.Post, newPathItem.Post},
			{"PUT", oldPathItem.Put, newPathItem.Put},
			{"DELETE", oldPathItem.Delete, newPathItem.Delete},
			{"PATCH", oldPathItem.Patch, newPathItem.Patch},
		}

		for _, m := range methodChecks {
			if m.oldOp == nil || m.newOp == nil {
				continue
			}

			responseChanges := e.compareResponses(m.oldOp.Responses, m.newOp.Responses, path, m.name)
			changes = append(changes, responseChanges...)
		}
	}

	return changes
}

// compareResponses compares response schemas between old and new operations.
func (e *CompatibilityEngine) compareResponses(oldResponses, newResponses *openapi3.Responses, path, method string) []*pb.BreakingChange {
	var changes []*pb.BreakingChange

	if oldResponses == nil || newResponses == nil {
		return changes
	}

	// Check for removed response codes
	for code, oldResp := range oldResponses.Map() {
		if oldResp == nil || oldResp.Value == nil {
			continue
		}

		newResp := newResponses.Value(code)
		if newResp == nil {
			changes = append(changes, &pb.BreakingChange{
				Type:        pb.BreakingChangeType_RESPONSE_SCHEMA_CHANGED,
				Path:        path,
				Method:      method,
				Description: fmt.Sprintf("Response for status code %s was removed from %s %s", code, method, path),
			})
			continue
		}

		// Check for removed required fields in response
		if oldResp.Value.Content != nil && newResp.Value != nil && newResp.Value.Content != nil {
			for mediaType, oldMediaType := range oldResp.Value.Content {
				newMediaType := newResp.Value.Content[mediaType]
				if newMediaType == nil {
					continue
				}

				fieldChanges := e.compareResponseSchemas(oldMediaType.Schema, newMediaType.Schema, path, method, code)
				changes = append(changes, fieldChanges...)
			}
		}
	}

	return changes
}

// compareResponseSchemas compares response schemas for type changes.
func (e *CompatibilityEngine) compareResponseSchemas(oldSchema, newSchema *openapi3.SchemaRef, path, method, statusCode string) []*pb.BreakingChange {
	var changes []*pb.BreakingChange

	if oldSchema == nil || newSchema == nil {
		return changes
	}
	if oldSchema.Value == nil || newSchema.Value == nil {
		return changes
	}

	// Check for type changes in response properties
	for propName, oldProp := range oldSchema.Value.Properties {
		if oldProp == nil || oldProp.Value == nil {
			continue
		}

		newProp := newSchema.Value.Properties[propName]

		// Property removed from response
		if newProp == nil || newProp.Value == nil {
			changes = append(changes, &pb.BreakingChange{
				Type:        pb.BreakingChangeType_RESPONSE_SCHEMA_CHANGED,
				Path:        path,
				Method:      method,
				Description: fmt.Sprintf("Field '%s' was removed from response (status %s) of %s %s", propName, statusCode, method, path),
			})
			continue
		}

		// Check type change
		if len(oldProp.Value.Type.Slice()) > 0 && len(newProp.Value.Type.Slice()) > 0 {
			oldType := oldProp.Value.Type.Slice()[0]
			newType := newProp.Value.Type.Slice()[0]
			if oldType != newType {
				changes = append(changes, &pb.BreakingChange{
					Type:        pb.BreakingChangeType_RESPONSE_SCHEMA_CHANGED,
					Path:        path,
					Method:      method,
					OldValue:    oldType,
					NewValue:    newType,
					Description: fmt.Sprintf("Type of field '%s' in response (status %s) changed from '%s' to '%s' in %s %s", propName, statusCode, oldType, newType, method, path),
				})
			}
		}
	}

	return changes
}

// isCompatibleTypeChange checks if a type change is backward compatible.
// For example, widening from int to number is typically allowed.
func isCompatibleTypeChange(oldType, newType string) bool {
	// Widening conversions that are generally safe
	compatibleChanges := map[string][]string{
		"integer": {"number"}, // int -> number is safe
		"int32":   {"int64", "integer", "number"},
		"int64":   {"number"},
		"float":   {"double", "number"},
	}

	if allowed, ok := compatibleChanges[oldType]; ok {
		for _, t := range allowed {
			if t == newType {
				return true
			}
		}
	}

	return false
}
