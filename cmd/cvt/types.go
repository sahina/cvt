package main

// breakingChangeOutput represents a breaking change for JSON output
type breakingChangeOutput struct {
	Type        string `json:"type"`
	Path        string `json:"path,omitempty"`
	Method      string `json:"method,omitempty"`
	Description string `json:"description"`
	OldValue    string `json:"old_value,omitempty"`
	NewValue    string `json:"new_value,omitempty"`
}

// consumerImpactOutput represents consumer impact for JSON output
type consumerImpactOutput struct {
	ConsumerID           string                 `json:"consumer_id"`
	ConsumerVersion      string                 `json:"consumer_version"`
	CurrentSchemaVersion string                 `json:"current_schema_version"`
	Environment          string                 `json:"environment"`
	WillBreak            bool                   `json:"will_break"`
	RelevantChanges      []breakingChangeOutput `json:"relevant_changes,omitempty"`
}
