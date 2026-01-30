package main

import (
	"encoding/json"
	"testing"
)

func TestBreakingChangeOutput_JSONMarshaling(t *testing.T) {
	bc := breakingChangeOutput{
		Type:        "ENDPOINT_REMOVED",
		Path:        "/users/{id}",
		Method:      "DELETE",
		Description: "Endpoint was removed",
		OldValue:    "existed",
		NewValue:    "",
	}

	data, err := json.Marshal(bc)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var unmarshaled breakingChangeOutput
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if unmarshaled.Type != bc.Type {
		t.Errorf("expected Type %q, got %q", bc.Type, unmarshaled.Type)
	}
	if unmarshaled.Path != bc.Path {
		t.Errorf("expected Path %q, got %q", bc.Path, unmarshaled.Path)
	}
	if unmarshaled.Method != bc.Method {
		t.Errorf("expected Method %q, got %q", bc.Method, unmarshaled.Method)
	}
	if unmarshaled.Description != bc.Description {
		t.Errorf("expected Description %q, got %q", bc.Description, unmarshaled.Description)
	}
}

func TestBreakingChangeOutput_OmitEmpty(t *testing.T) {
	bc := breakingChangeOutput{
		Type:        "FIELD_REMOVED",
		Description: "Field was removed",
		// Path, Method, OldValue, NewValue are empty
	}

	data, err := json.Marshal(bc)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Check that empty fields are omitted
	jsonStr := string(data)
	if contains(jsonStr, "path") {
		t.Error("expected empty path to be omitted from JSON")
	}
	if contains(jsonStr, "method") {
		t.Error("expected empty method to be omitted from JSON")
	}
	if contains(jsonStr, "old_value") {
		t.Error("expected empty old_value to be omitted from JSON")
	}
	if contains(jsonStr, "new_value") {
		t.Error("expected empty new_value to be omitted from JSON")
	}
}

func TestConsumerImpactOutput_JSONMarshaling(t *testing.T) {
	ci := consumerImpactOutput{
		ConsumerID:           "consumer-1",
		ConsumerVersion:      "1.0.0",
		CurrentSchemaVersion: "2.0.0",
		Environment:          "prod",
		WillBreak:            true,
		RelevantChanges: []breakingChangeOutput{
			{
				Type:        "ENDPOINT_REMOVED",
				Path:        "/users",
				Method:      "GET",
				Description: "Endpoint removed",
			},
		},
	}

	data, err := json.Marshal(ci)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var unmarshaled consumerImpactOutput
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if unmarshaled.ConsumerID != ci.ConsumerID {
		t.Errorf("expected ConsumerID %q, got %q", ci.ConsumerID, unmarshaled.ConsumerID)
	}
	if unmarshaled.WillBreak != ci.WillBreak {
		t.Errorf("expected WillBreak %v, got %v", ci.WillBreak, unmarshaled.WillBreak)
	}
	if len(unmarshaled.RelevantChanges) != 1 {
		t.Errorf("expected 1 relevant change, got %d", len(unmarshaled.RelevantChanges))
	}
}

func TestConsumerImpactOutput_EmptyRelevantChanges(t *testing.T) {
	ci := consumerImpactOutput{
		ConsumerID:      "consumer-1",
		ConsumerVersion: "1.0.0",
		WillBreak:       false,
		// RelevantChanges is empty
	}

	data, err := json.Marshal(ci)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Check that empty relevant_changes is omitted
	jsonStr := string(data)
	if contains(jsonStr, "relevant_changes") {
		t.Error("expected empty relevant_changes to be omitted from JSON")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
