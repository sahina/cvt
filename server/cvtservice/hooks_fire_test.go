package cvtservice

import (
	"context"
	"sync"
	"testing"

	"github.com/sahina/cvt/pkg/cvt"
	eventspb "github.com/sahina/cvt/pkg/cvtplugin/pb/events/v1"
	registrypb "github.com/sahina/cvt/pkg/cvtplugin/pb/registry/v1"
	"github.com/sahina/cvt/server/pb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingHooks is a Hooks impl that captures every call. Tests use it to
// assert what core fired without spinning up a real plugin subprocess.
type recordingHooks struct {
	mu                    sync.Mutex
	breakingChangeCalls   []*eventspb.BreakingChangeDetectedRequest
	registerConsumerCalls []*registrypb.RegisterConsumerUsageRequest
	validationFailedCalls []*eventspb.ValidationFailedRequest
	fetchSchemaCalls      []*registrypb.FetchSchemaRequest
}

func (r *recordingHooks) FetchSchema(_ context.Context, req *registrypb.FetchSchemaRequest) (*registrypb.FetchSchemaResponse, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fetchSchemaCalls = append(r.fetchSchemaCalls, req)
	return nil, nil
}

func (r *recordingHooks) RegisterConsumerUsage(_ context.Context, req *registrypb.RegisterConsumerUsageRequest) (*registrypb.RegisterConsumerUsageResponse, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.registerConsumerCalls = append(r.registerConsumerCalls, req)
	return &registrypb.RegisterConsumerUsageResponse{Acknowledged: true}, nil
}

func (r *recordingHooks) OnBreakingChangeDetected(_ context.Context, req *eventspb.BreakingChangeDetectedRequest) (*eventspb.EventResponse, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.breakingChangeCalls = append(r.breakingChangeCalls, req)
	return &eventspb.EventResponse{Acknowledged: true}, nil
}

func (r *recordingHooks) OnValidationFailed(_ context.Context, req *eventspb.ValidationFailedRequest) (*eventspb.EventResponse, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.validationFailedCalls = append(r.validationFailedCalls, req)
	return &eventspb.EventResponse{Acknowledged: true}, nil
}

// Compile-time assertion that recordingHooks satisfies cvt.Hooks.
var _ cvt.Hooks = (*recordingHooks)(nil)

func TestFireOnBreakingChangeDetected_NoopHooks_NoPanic(t *testing.T) {
	s, err := NewValidatorService()
	require.NoError(t, err)
	defer s.Close()
	// Default hooks is nil → NoopHooks via hooksOrNoop. Must not panic.
	s.fireOnBreakingChangeDetected(context.Background(), "test", "1.0.0", "2.0.0", []*pb.BreakingChange{
		{Type: pb.BreakingChangeType_ENDPOINT_REMOVED, Path: "/x", Method: "GET"},
	}, "CompareSchemas")
}

func TestFireOnBreakingChangeDetected_EmptyChangesSkipped(t *testing.T) {
	s, err := NewValidatorService()
	require.NoError(t, err)
	defer s.Close()
	rec := &recordingHooks{}
	s.SetHooks(rec)
	s.fireOnBreakingChangeDetected(context.Background(), "test", "1.0.0", "2.0.0", nil, "CompareSchemas")
	assert.Empty(t, rec.breakingChangeCalls, "empty changes must skip the hook")
}

func TestFireOnBreakingChangeDetected_HappyPath(t *testing.T) {
	s, err := NewValidatorService()
	require.NoError(t, err)
	defer s.Close()
	rec := &recordingHooks{}
	s.SetHooks(rec)

	changes := []*pb.BreakingChange{
		{Type: pb.BreakingChangeType_ENDPOINT_REMOVED, Path: "/users", Method: "GET", Description: "GET /users removed"},
		{Type: pb.BreakingChangeType_REQUIRED_FIELD_ADDED, Path: "/pets", Method: "POST", Description: "required field added"},
	}
	s.fireOnBreakingChangeDetected(context.Background(), "pet-api", "1.0.0", "2.0.0", changes, "RegisterSchema")

	require.Len(t, rec.breakingChangeCalls, 1)
	got := rec.breakingChangeCalls[0]
	assert.Equal(t, "pet-api", got.SchemaId)
	assert.Equal(t, "1.0.0", got.OldVersion)
	assert.Equal(t, "2.0.0", got.NewVersion)
	assert.Equal(t, "RegisterSchema", got.DetectedBy)
	require.Len(t, got.Changes, 2)
	assert.Equal(t, "endpoint_removed", got.Changes[0].Kind)
	assert.Equal(t, "required_field_added", got.Changes[1].Kind)
}

func TestFireRegisterConsumerUsage_NoopHooks_NoPanic(t *testing.T) {
	s, err := NewValidatorService()
	require.NoError(t, err)
	defer s.Close()
	s.fireRegisterConsumerUsage(context.Background(), &pb.RegisterConsumerRequest{
		ConsumerId: "order-service", SchemaId: "pet-api",
	})
}

func TestFireRegisterConsumerUsage_FieldMapping(t *testing.T) {
	s, err := NewValidatorService()
	require.NoError(t, err)
	defer s.Close()
	rec := &recordingHooks{}
	s.SetHooks(rec)

	s.fireRegisterConsumerUsage(context.Background(), &pb.RegisterConsumerRequest{
		ConsumerId:      "order-service",
		ConsumerVersion: "1.2.3",
		SchemaId:        "pet-api",
		SchemaVersion:   "2.1.0",
		Environment:     "ci",
		UsedEndpoints: []*pb.EndpointUsage{
			{Method: "GET", Path: "/pets/{id}", UsedFields: []string{"id", "name"}},
		},
	})

	require.Len(t, rec.registerConsumerCalls, 1)
	got := rec.registerConsumerCalls[0]
	assert.Equal(t, "order-service", got.ConsumerId)
	assert.Equal(t, "pet-api", got.SchemaId)
	assert.Equal(t, "2.1.0", got.SchemaVersion)
	assert.Equal(t, "ci", got.Environment)
	require.Len(t, got.Endpoints, 1)
	assert.Equal(t, "GET", got.Endpoints[0].Method)
	assert.Equal(t, "/pets/{id}", got.Endpoints[0].Path)
	assert.Equal(t, []string{"id", "name"}, got.Endpoints[0].UsedFields, "used_fields must propagate per plugin proto v1.1")
}
