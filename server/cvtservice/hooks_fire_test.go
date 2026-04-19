package cvtservice

import (
	"context"
	"sync"
	"testing"

	"github.com/sahina/cvt/pkg/cvt"
	eventspb "github.com/sahina/cvt/pkg/cvtplugin/pb/events/v1"
	registrypb "github.com/sahina/cvt/pkg/cvtplugin/pb/registry/v1"
	"github.com/sahina/cvt/server/pb"
	"github.com/sahina/cvt/server/storage"
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
	fetchSchemaResp       *registrypb.FetchSchemaResponse
	fetchSchemaErr        error
}

func (r *recordingHooks) FetchSchema(_ context.Context, req *registrypb.FetchSchemaRequest) (*registrypb.FetchSchemaResponse, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fetchSchemaCalls = append(r.fetchSchemaCalls, req)
	if r.fetchSchemaResp != nil || r.fetchSchemaErr != nil {
		return r.fetchSchemaResp, r.fetchSchemaErr
	}
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

const fetchSchemaTestSpec = `{"openapi":"3.0.0","info":{"title":"Test","version":"1.0.0"},"paths":{}}`

func TestTryFetchSchemaFromPlugin_NoopHooks_FallsThrough(t *testing.T) {
	s, err := NewValidatorService()
	require.NoError(t, err)
	defer s.Close()
	// No hooks set → NoopHooks.FetchSchema returns (nil, nil) → helper
	// signals "not found, no error" so getSchemaEntry moves on to storage.
	entry, ok, fetchErr := s.tryFetchSchemaFromPlugin(context.Background(), "pet-api", "")
	assert.Nil(t, entry)
	assert.False(t, ok)
	assert.NoError(t, fetchErr)
}

func TestTryFetchSchemaFromPlugin_HappyPath_CachesEntry(t *testing.T) {
	s, err := NewValidatorService()
	require.NoError(t, err)
	defer s.Close()
	rec := &recordingHooks{
		fetchSchemaResp: &registrypb.FetchSchemaResponse{
			Spec:            []byte(fetchSchemaTestSpec),
			ResolvedVersion: "1.0.0",
		},
	}
	s.SetHooks(rec)

	entry, ok, fetchErr := s.tryFetchSchemaFromPlugin(context.Background(), "pet-api", "")
	require.NoError(t, fetchErr)
	require.True(t, ok)
	require.NotNil(t, entry)
	assert.Equal(t, "1.0.0", entry.Metadata.SchemaVersion)
	assert.NotNil(t, entry.Router, "router must be built for plugin-supplied specs")

	require.Len(t, rec.fetchSchemaCalls, 1)
	assert.Equal(t, "pet-api", rec.fetchSchemaCalls[0].SchemaId)
	assert.Equal(t, "", rec.fetchSchemaCalls[0].Version, "empty version propagates as 'latest'")

	// Entry must land in the bare (unversioned) slot when the lookup was
	// unversioned — mirrors the storage-readthrough rule.
	cached, cachedOK := s.cache.Get("pet-api")
	assert.True(t, cachedOK)
	assert.Same(t, entry, cached)
}

func TestTryFetchSchemaFromPlugin_VersionedLookup_UsesVersionedSlot(t *testing.T) {
	s, err := NewValidatorService()
	require.NoError(t, err)
	defer s.Close()
	rec := &recordingHooks{
		fetchSchemaResp: &registrypb.FetchSchemaResponse{
			Spec:            []byte(fetchSchemaTestSpec),
			ResolvedVersion: "2.0.0",
		},
	}
	s.SetHooks(rec)

	_, ok, _ := s.tryFetchSchemaFromPlugin(context.Background(), "pet-api", "2.0.0")
	require.True(t, ok)
	require.Len(t, rec.fetchSchemaCalls, 1)
	assert.Equal(t, "2.0.0", rec.fetchSchemaCalls[0].Version)

	// Versioned-only cache write: the bare key must stay empty so an old
	// version can't masquerade as "latest".
	_, bareHit := s.cache.Get("pet-api")
	assert.False(t, bareHit)
	_, versionHit := s.cache.GetVersion("pet-api", "2.0.0")
	assert.True(t, versionHit)
}

func TestTryFetchSchemaFromPlugin_MalformedSpec_FallsThrough(t *testing.T) {
	s, err := NewValidatorService()
	require.NoError(t, err)
	defer s.Close()
	rec := &recordingHooks{
		fetchSchemaResp: &registrypb.FetchSchemaResponse{Spec: []byte("{not openapi")},
	}
	s.SetHooks(rec)

	entry, ok, fetchErr := s.tryFetchSchemaFromPlugin(context.Background(), "pet-api", "")
	assert.Nil(t, entry)
	assert.False(t, ok)
	assert.NoError(t, fetchErr, "malformed spec logs+falls through, does not surface error")
}

func TestTryFetchSchemaFromPlugin_PluginError_Surfaces(t *testing.T) {
	s, err := NewValidatorService()
	require.NoError(t, err)
	defer s.Close()
	rec := &recordingHooks{fetchSchemaErr: assert.AnError}
	s.SetHooks(rec)

	entry, ok, fetchErr := s.tryFetchSchemaFromPlugin(context.Background(), "pet-api", "")
	assert.Nil(t, entry)
	assert.False(t, ok)
	assert.ErrorIs(t, fetchErr, assert.AnError, "fail_closed plugin error must reach caller so storage is not consulted")
}

func TestGetSchemaEntry_FetchesFromPluginBeforeStorage(t *testing.T) {
	// Prove ordering — not just that the plugin works. Populate storage
	// with one spec, have the plugin return a *different* spec, show that
	// getSchemaEntry returns the plugin's version.
	memStore := storage.NewMemoryStore()
	s, err := NewValidatorServiceWithStore(memStore)
	require.NoError(t, err)
	defer s.Close()

	const storageSpec = `{"openapi":"3.0.0","info":{"title":"FromStorage","version":"0.9.0"},"paths":{}}`
	const pluginSpec = `{"openapi":"3.0.0","info":{"title":"FromPlugin","version":"2.0.0"},"paths":{}}`

	_, err = s.RegisterSchema(context.Background(), &pb.RegisterSchemaRequest{
		SchemaId:      "pet-api",
		SchemaContent: storageSpec,
	})
	require.NoError(t, err)
	s.cache.Delete("pet-api") // force the miss path

	rec := &recordingHooks{
		fetchSchemaResp: &registrypb.FetchSchemaResponse{
			Spec:            []byte(pluginSpec),
			ResolvedVersion: "2.0.0",
		},
	}
	s.SetHooks(rec)

	entry, found := s.getSchemaEntry(context.Background(), "pet-api", "")
	require.True(t, found)
	require.NotNil(t, entry)
	assert.Equal(t, "2.0.0", entry.Metadata.SchemaVersion, "plugin spec must win over storage")
	assert.Equal(t, "FromPlugin", entry.Document.Info.Title)
	require.Len(t, rec.fetchSchemaCalls, 1)
}
