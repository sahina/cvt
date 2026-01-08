package cvt

import (
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// BreakingChangeType identifies the type of breaking change.
type BreakingChangeType string

const (
	BreakingChangeEndpointRemoved        BreakingChangeType = "ENDPOINT_REMOVED"
	BreakingChangeRequiredFieldAdded     BreakingChangeType = "REQUIRED_FIELD_ADDED"
	BreakingChangeTypeChanged            BreakingChangeType = "TYPE_CHANGED"
	BreakingChangeRequiredParameterAdded BreakingChangeType = "REQUIRED_PARAMETER_ADDED"
	BreakingChangeResponseSchemaChanged  BreakingChangeType = "RESPONSE_SCHEMA_CHANGED"
	BreakingChangeEnumValueRemoved       BreakingChangeType = "ENUM_VALUE_REMOVED"
)

// BreakingChange represents a detected breaking change between schemas.
type BreakingChange struct {
	Type        BreakingChangeType `json:"type"`
	Path        string             `json:"path"`
	Method      string             `json:"method"`
	Description string             `json:"description"`
	OldValue    string             `json:"old_value,omitempty"`
	NewValue    string             `json:"new_value,omitempty"`
}

// CompatibilityEngine checks for breaking changes between schema versions.
type CompatibilityEngine struct{}

// NewCompatibilityEngine creates a new CompatibilityEngine.
func NewCompatibilityEngine() *CompatibilityEngine {
	return &CompatibilityEngine{}
}

// CompareSchemas compares two OpenAPI documents and detects breaking changes.
func (e *CompatibilityEngine) CompareSchemas(oldDoc, newDoc *openapi3.T) ([]*BreakingChange, bool) {
	var changes []*BreakingChange

	changes = append(changes, e.checkRemovedEndpoints(oldDoc, newDoc)...)
	changes = append(changes, e.checkAddedRequiredParameters(oldDoc, newDoc)...)
	changes = append(changes, e.checkAddedRequiredBodyFields(oldDoc, newDoc)...)
	changes = append(changes, e.checkTypeChanges(oldDoc, newDoc)...)
	changes = append(changes, e.checkRemovedEnumValues(oldDoc, newDoc)...)
	changes = append(changes, e.checkResponseChanges(oldDoc, newDoc)...)

	return changes, len(changes) == 0
}

func (e *CompatibilityEngine) checkRemovedEndpoints(oldDoc, newDoc *openapi3.T) []*BreakingChange {
	var changes []*BreakingChange

	if oldDoc.Paths == nil {
		return changes
	}

	for path, oldPathItem := range oldDoc.Paths.Map() {
		var newPathItem *openapi3.PathItem
		if newDoc.Paths != nil {
			newPathItem = newDoc.Paths.Find(path)
		}

		methods := []struct {
			name     string
			oldOp    *openapi3.Operation
			getNewOp func(*openapi3.PathItem) *openapi3.Operation
		}{
			{"GET", oldPathItem.Get, func(p *openapi3.PathItem) *openapi3.Operation { return p.Get }},
			{"POST", oldPathItem.Post, func(p *openapi3.PathItem) *openapi3.Operation { return p.Post }},
			{"PUT", oldPathItem.Put, func(p *openapi3.PathItem) *openapi3.Operation { return p.Put }},
			{"DELETE", oldPathItem.Delete, func(p *openapi3.PathItem) *openapi3.Operation { return p.Delete }},
			{"PATCH", oldPathItem.Patch, func(p *openapi3.PathItem) *openapi3.Operation { return p.Patch }},
		}

		for _, m := range methods {
			if m.oldOp == nil {
				continue
			}

			exists := false
			if newPathItem != nil {
				if op := m.getNewOp(newPathItem); op != nil {
					exists = true
				}
			}

			if !exists {
				changes = append(changes, &BreakingChange{
					Type:        BreakingChangeEndpointRemoved,
					Path:        path,
					Method:      m.name,
					Description: fmt.Sprintf("Endpoint %s %s was removed", m.name, path),
				})
			}
		}
	}

	return changes
}

func (e *CompatibilityEngine) checkAddedRequiredParameters(oldDoc, newDoc *openapi3.T) []*BreakingChange {
	var changes []*BreakingChange

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

			oldParams := make(map[string]bool)
			for _, param := range m.oldOp.Parameters {
				if param.Value != nil {
					oldParams[param.Value.Name] = true
				}
			}

			for _, param := range m.newOp.Parameters {
				if param.Value == nil {
					continue
				}
				if param.Value.Required && !oldParams[param.Value.Name] {
					changes = append(changes, &BreakingChange{
						Type:        BreakingChangeRequiredParameterAdded,
						Path:        path,
						Method:      m.name,
						Description: fmt.Sprintf("Required parameter '%s' was added to %s %s", param.Value.Name, m.name, path),
					})
				}
			}
		}
	}

	return changes
}

func (e *CompatibilityEngine) checkAddedRequiredBodyFields(oldDoc, newDoc *openapi3.T) []*BreakingChange {
	var changes []*BreakingChange

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

func (e *CompatibilityEngine) compareRequestBodies(oldBody, newBody *openapi3.RequestBodyRef, path, method string) []*BreakingChange {
	var changes []*BreakingChange

	if newBody == nil || newBody.Value == nil {
		return changes
	}

	newContent := newBody.Value.Content
	if newContent == nil {
		return changes
	}

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

	for _, mediaType := range newContent {
		if mediaType.Schema == nil || mediaType.Schema.Value == nil {
			continue
		}

		for _, field := range mediaType.Schema.Value.Required {
			if !oldRequired[field] {
				changes = append(changes, &BreakingChange{
					Type:        BreakingChangeRequiredFieldAdded,
					Path:        path,
					Method:      method,
					Description: fmt.Sprintf("Required field '%s' was added to request body of %s %s", field, method, path),
				})
			}
		}
	}

	return changes
}

func (e *CompatibilityEngine) checkTypeChanges(oldDoc, newDoc *openapi3.T) []*BreakingChange {
	var changes []*BreakingChange

	if oldDoc.Paths == nil || newDoc.Paths == nil {
		return changes
	}

	for path, newPathItem := range newDoc.Paths.Map() {
		oldPathItem := oldDoc.Paths.Find(path)
		if oldPathItem == nil {
			continue
		}

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

			paramChanges := e.checkParameterTypeChanges(m.oldOp, m.newOp, path, m.name)
			changes = append(changes, paramChanges...)
		}
	}

	return changes
}

func (e *CompatibilityEngine) checkParameterTypeChanges(oldOp, newOp *openapi3.Operation, path, method string) []*BreakingChange {
	var changes []*BreakingChange

	oldParams := make(map[string]string)
	for _, param := range oldOp.Parameters {
		if param.Value != nil && param.Value.Schema != nil && param.Value.Schema.Value != nil {
			if len(param.Value.Schema.Value.Type.Slice()) > 0 {
				oldParams[param.Value.Name] = param.Value.Schema.Value.Type.Slice()[0]
			}
		}
	}

	for _, param := range newOp.Parameters {
		if param.Value == nil || param.Value.Schema == nil || param.Value.Schema.Value == nil {
			continue
		}

		oldType, exists := oldParams[param.Value.Name]
		if !exists {
			continue
		}

		newType := ""
		if len(param.Value.Schema.Value.Type.Slice()) > 0 {
			newType = param.Value.Schema.Value.Type.Slice()[0]
		}

		if oldType != newType && !isCompatibleTypeChange(oldType, newType) {
			changes = append(changes, &BreakingChange{
				Type:        BreakingChangeTypeChanged,
				Path:        path,
				Method:      method,
				OldValue:    oldType,
				NewValue:    newType,
				Description: fmt.Sprintf("Type of parameter '%s' changed from '%s' to '%s' in %s %s", param.Value.Name, oldType, newType, method, path),
			})
		}
	}

	return changes
}

func (e *CompatibilityEngine) checkRemovedEnumValues(oldDoc, newDoc *openapi3.T) []*BreakingChange {
	var changes []*BreakingChange

	if oldDoc.Paths == nil || newDoc.Paths == nil {
		return changes
	}

	for path, newPathItem := range newDoc.Paths.Map() {
		oldPathItem := oldDoc.Paths.Find(path)
		if oldPathItem == nil {
			continue
		}

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

			enumChanges := e.checkParameterEnumChanges(m.oldOp, m.newOp, path, m.name)
			changes = append(changes, enumChanges...)
		}
	}

	return changes
}

func (e *CompatibilityEngine) checkParameterEnumChanges(oldOp, newOp *openapi3.Operation, path, method string) []*BreakingChange {
	var changes []*BreakingChange

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

	for _, param := range newOp.Parameters {
		if param.Value == nil || param.Value.Schema == nil || param.Value.Schema.Value == nil {
			continue
		}

		oldEnumSet, exists := oldEnums[param.Value.Name]
		if !exists || len(param.Value.Schema.Value.Enum) == 0 {
			continue
		}

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
			changes = append(changes, &BreakingChange{
				Type:        BreakingChangeEnumValueRemoved,
				Path:        path,
				Method:      method,
				OldValue:    strings.Join(removedValues, ", "),
				Description: fmt.Sprintf("Enum values [%s] were removed from parameter '%s' in %s %s", strings.Join(removedValues, ", "), param.Value.Name, method, path),
			})
		}
	}

	return changes
}

func (e *CompatibilityEngine) checkResponseChanges(oldDoc, newDoc *openapi3.T) []*BreakingChange {
	var changes []*BreakingChange

	if oldDoc.Paths == nil || newDoc.Paths == nil {
		return changes
	}

	for path, newPathItem := range newDoc.Paths.Map() {
		oldPathItem := oldDoc.Paths.Find(path)
		if oldPathItem == nil {
			continue
		}

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

func (e *CompatibilityEngine) compareResponses(oldResponses, newResponses *openapi3.Responses, path, method string) []*BreakingChange {
	var changes []*BreakingChange

	if oldResponses == nil || newResponses == nil {
		return changes
	}

	for code, oldResp := range oldResponses.Map() {
		if oldResp == nil || oldResp.Value == nil {
			continue
		}

		newResp := newResponses.Value(code)
		if newResp == nil {
			changes = append(changes, &BreakingChange{
				Type:        BreakingChangeResponseSchemaChanged,
				Path:        path,
				Method:      method,
				Description: fmt.Sprintf("Response for status code %s was removed from %s %s", code, method, path),
			})
		}
	}

	return changes
}

func isCompatibleTypeChange(oldType, newType string) bool {
	compatibleChanges := map[string][]string{
		"integer": {"number"},
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
