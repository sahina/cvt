package cvt

import (
	"context"

	eventspb "github.com/sahina/cvt/pkg/cvtplugin/pb/events/v1"
)

// fireOnValidationFailed dispatches the on_validation_failed hook. Runs
// synchronously inline with Validate so the plugin's timeout + on_error
// policy apply. Errors from the hook are intentionally ignored here — the
// caller of Validate already got their validation result; hook failure is
// a separate observability concern reported via the hook adapter's audit
// + metrics path.
//
// Request_id is empty: pkg/cvt doesn't carry one. Callers that want
// correlation across plugin audit entries should wrap Validate and
// populate request_id themselves via the Hooks implementation.
func (v *Validator) fireOnValidationFailed(schemaID string, interaction *Interaction, result *ValidationResult) {
	h := v.hooksOrNoop()
	req := &eventspb.ValidationFailedRequest{
		SchemaId: schemaID,
		Method:   interaction.Method,
		Path:     interaction.Path,
	}
	for _, msg := range result.Errors {
		req.Errors = append(req.Errors, &eventspb.ValidationError{
			Description: msg,
		})
	}
	// Background context: Validate has no ctx parameter today. Plugin
	// call still bounded by the per-plugin timeout enforced in the Hooks
	// adapter.
	_, _ = h.OnValidationFailed(context.Background(), req)
}
