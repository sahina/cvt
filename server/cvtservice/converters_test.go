package cvtservice

import (
	"testing"

	"github.com/sahina/cvt/server/pb"
	"github.com/stretchr/testify/assert"
)

func TestConvertBreakingChanges_AllFields(t *testing.T) {
	in := []*pb.BreakingChange{
		{
			Type:        pb.BreakingChangeType_ENDPOINT_REMOVED,
			Path:        "/users/{id}",
			Method:      "DELETE",
			Description: "DELETE /users/{id} removed",
			OldValue:    "endpoint",
			NewValue:    "",
		},
	}
	out := convertBreakingChanges(in)
	require := assert.New(t)
	require.Len(out, 1)
	require.Equal("endpoint_removed", out[0].Kind)
	require.Equal("/users/{id}", out[0].Path)
	require.Equal("DELETE", out[0].Method)
	require.Equal("DELETE /users/{id} removed", out[0].Description)
	// FieldPath intentionally not populated.
	require.Empty(out[0].FieldPath)
}

func TestConvertBreakingChanges_EmptyInput(t *testing.T) {
	assert.Nil(t, convertBreakingChanges(nil))
	assert.Nil(t, convertBreakingChanges([]*pb.BreakingChange{}))
}

// TestConvertBC_UnknownEnum: the converter must not panic on a future
// enum value; it must produce a deterministic string and (we assume) log
// a WARN. This is decision 1D from the eng review.
func TestConvertBC_UnknownEnum(t *testing.T) {
	in := []*pb.BreakingChange{
		{Type: pb.BreakingChangeType(999), Description: "future kind"},
	}
	out := convertBreakingChanges(in)
	require := assert.New(t)
	require.Len(out, 1)
	require.Equal("BREAKING_CHANGE_TYPE_999", out[0].Kind)
	require.Equal("future kind", out[0].Description)
}

func TestConvertEndpointUsage_PreservesAllFields(t *testing.T) {
	in := []*pb.EndpointUsage{
		{
			Method:     "GET",
			Path:       "/pets/{id}",
			UsedFields: []string{"id", "name", "tags"},
		},
	}
	out := convertEndpointUsage(in)
	require := assert.New(t)
	require.Len(out, 1)
	require.Equal("GET", out[0].Method)
	require.Equal("/pets/{id}", out[0].Path)
	require.Equal([]string{"id", "name", "tags"}, out[0].UsedFields)
}

func TestConvertEndpointUsage_EmptyInput(t *testing.T) {
	assert.Nil(t, convertEndpointUsage(nil))
	assert.Nil(t, convertEndpointUsage([]*pb.EndpointUsage{}))
}
