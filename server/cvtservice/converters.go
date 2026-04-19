package cvtservice

import (
	"fmt"

	eventspb "github.com/sahina/cvt/pkg/cvtplugin/pb/events/v1"
	registrypb "github.com/sahina/cvt/pkg/cvtplugin/pb/registry/v1"
	"github.com/sahina/cvt/server/pb"
	"go.uber.org/zap"
)

// convertBreakingChanges maps server-side pb.BreakingChange (uses an enum
// kind) to plugin-side eventspb.BreakingChange (uses a string kind).
//
// Forward-compatible: an enum value the converter doesn't recognize
// produces the string `BREAKING_CHANGE_TYPE_<N>` and a server-side WARN
// log so we notice when proto evolves and the converter falls behind. The
// RPC still succeeds (no panic) — the plugin gets a deterministic string
// it can ignore or alert on.
//
// Per decision 1D (eng review 2026-04-18). FieldPath stays empty in v1
// because the server proto doesn't carry the field; plugin proto v1.2 may
// pass it through.
func convertBreakingChanges(in []*pb.BreakingChange) []*eventspb.BreakingChange {
	if len(in) == 0 {
		return nil
	}
	out := make([]*eventspb.BreakingChange, len(in))
	for i, c := range in {
		out[i] = &eventspb.BreakingChange{
			Kind:        breakingChangeKind(c.Type),
			Description: c.Description,
			Method:      c.Method,
			Path:        c.Path,
			// FieldPath: intentionally empty — server proto has no equivalent field.
		}
	}
	return out
}

// breakingChangeKind translates a server BreakingChangeType enum to the
// snake-case string the plugin contract expects. Unknown values fall
// through to BREAKING_CHANGE_TYPE_<N> and trigger a WARN log so the
// converter is updated before plugins start dropping unknown kinds.
func breakingChangeKind(t pb.BreakingChangeType) string {
	switch t {
	case pb.BreakingChangeType_BREAKING_CHANGE_UNSPECIFIED:
		return "unspecified"
	case pb.BreakingChangeType_ENDPOINT_REMOVED:
		return "endpoint_removed"
	case pb.BreakingChangeType_REQUIRED_FIELD_ADDED:
		return "required_field_added"
	case pb.BreakingChangeType_TYPE_CHANGED:
		return "type_changed"
	case pb.BreakingChangeType_REQUIRED_PARAMETER_ADDED:
		return "required_parameter_added"
	case pb.BreakingChangeType_RESPONSE_SCHEMA_CHANGED:
		return "response_schema_changed"
	case pb.BreakingChangeType_ENUM_VALUE_REMOVED:
		return "enum_value_removed"
	default:
		Warn("unknown BreakingChangeType in converter — update server/cvtservice/converters.go",
			zap.Int32("enum_value", int32(t)))
		return fmt.Sprintf("BREAKING_CHANGE_TYPE_%d", int32(t))
	}
}

// convertEndpointUsage maps server-side pb.EndpointUsage (carries
// used_fields) to plugin-side registrypb.EndpointUsage. As of plugin
// proto v1.1, used_fields propagates end-to-end (was dropped in v1.0).
func convertEndpointUsage(in []*pb.EndpointUsage) []*registrypb.EndpointUsage {
	if len(in) == 0 {
		return nil
	}
	out := make([]*registrypb.EndpointUsage, len(in))
	for i, e := range in {
		out[i] = &registrypb.EndpointUsage{
			Method:     e.Method,
			Path:       e.Path,
			UsedFields: e.UsedFields,
		}
	}
	return out
}
