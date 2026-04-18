package pluginclient

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sahina/cvt/internal/pluginmgr"
	eventspb "github.com/sahina/cvt/pkg/cvtplugin/pb/events/v1"
	registrypb "github.com/sahina/cvt/pkg/cvtplugin/pb/registry/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// These tests cover the on_error policy, timeout → OutcomeTimeout
// mapping, nil-response fallback, and codes.Unimplemented passthrough
// via fakes. No subprocess involved.

// fakeRegistryClient implements registrypb.RegistryProviderClient in memory.
type fakeRegistryClient struct {
	fetchResp      *registrypb.FetchSchemaResponse
	fetchErr       error
	fetchDelay     time.Duration
	registerResp   *registrypb.RegisterConsumerUsageResponse
	registerErr    error
	fetchCalls     int32
	registerCalls  int32
}

func (f *fakeRegistryClient) FetchSchema(ctx context.Context, _ *registrypb.FetchSchemaRequest, _ ...grpc.CallOption) (*registrypb.FetchSchemaResponse, error) {
	atomic.AddInt32(&f.fetchCalls, 1)
	if f.fetchDelay > 0 {
		select {
		case <-time.After(f.fetchDelay):
		case <-ctx.Done():
			// Real gRPC wraps ctx.Err() in a gRPC status. Mirror that.
			return nil, status.FromContextError(ctx.Err()).Err()
		}
	}
	return f.fetchResp, f.fetchErr
}

func (f *fakeRegistryClient) RegisterConsumerUsage(_ context.Context, _ *registrypb.RegisterConsumerUsageRequest, _ ...grpc.CallOption) (*registrypb.RegisterConsumerUsageResponse, error) {
	atomic.AddInt32(&f.registerCalls, 1)
	return f.registerResp, f.registerErr
}

type fakeEventClient struct {
	breakErr     error
	validateErr  error
	breakResp    *eventspb.EventResponse
	validateResp *eventspb.EventResponse
}

func (f *fakeEventClient) OnBreakingChangeDetected(_ context.Context, _ *eventspb.BreakingChangeDetectedRequest, _ ...grpc.CallOption) (*eventspb.EventResponse, error) {
	return f.breakResp, f.breakErr
}
func (f *fakeEventClient) OnValidationFailed(_ context.Context, _ *eventspb.ValidationFailedRequest, _ ...grpc.CallOption) (*eventspb.EventResponse, error) {
	return f.validateResp, f.validateErr
}

// buildManagerWithFakes produces a Manager whose Registry(name) /
// Events(name) return the provided fakes, without forking a subprocess.
// The manager's internal handles map is populated via a test-only helper
// method we define below.
func buildManagerWithFakes(t *testing.T, onError string, reg *fakeRegistryClient, ev *fakeEventClient) (*pluginmgr.Manager, *pluginmgr.RecordingAuditSink) {
	t.Helper()
	cfg := &pluginmgr.Config{
		ConfigVersion: 1,
		Plugins: map[string]pluginmgr.PluginConfig{
			"test": {
				Binary:  "/tmp/ignored",
				Timeout: 1 * time.Second,
				OnError: onError,
			},
		},
		Hooks: pluginmgr.HookBindings{
			FetchSchema:              "test",
			RegisterConsumerUsage:    "test",
			OnBreakingChangeDetected: "test",
			OnValidationFailed:       "test",
		},
	}
	audit := &pluginmgr.RecordingAuditSink{}
	mgr := pluginmgr.NewForTest(cfg, pluginmgr.Options{Audit: audit})
	mgr.InjectClientsForTest("test", reg, ev, pluginmgr.HandleInfo{
		Name: "test", ReportedVersion: "test/0.0.1", SHA256: "abc123def456", PID: 1,
	})
	return mgr, audit
}

func TestFetchSchema_HappyPath(t *testing.T) {
	reg := &fakeRegistryClient{
		fetchResp: &registrypb.FetchSchemaResponse{Spec: []byte("openapi"), ResolvedVersion: "1.0.0"},
	}
	mgr, audit := buildManagerWithFakes(t, pluginmgr.OnErrorFailClosed, reg, nil)
	a := NewHooks(mgr)

	resp, err := a.FetchSchema(context.Background(), &registrypb.FetchSchemaRequest{SchemaId: "pet-api", RequestId: "r1"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "1.0.0", resp.GetResolvedVersion())
	assert.Equal(t, int32(1), reg.fetchCalls)
	require.Len(t, audit.Records, 1)
	assert.Equal(t, pluginmgr.OutcomeOK, audit.Records[0].Outcome)
	assert.Equal(t, "r1", audit.Records[0].RequestID)
	assert.Equal(t, pluginmgr.AuditKindRead, audit.Records[0].Kind)
}

func TestFetchSchema_NilResponseFallsBackToNoop(t *testing.T) {
	// Plugin returns (nil, nil) — valid gRPC but the core code used to
	// NPE on type assertion. Now falls through to NoopHooks.
	reg := &fakeRegistryClient{fetchResp: nil, fetchErr: nil}
	mgr, _ := buildManagerWithFakes(t, pluginmgr.OnErrorFailClosed, reg, nil)
	a := NewHooks(mgr)

	resp, err := a.FetchSchema(context.Background(), &registrypb.FetchSchemaRequest{SchemaId: "pet-api"})
	require.NoError(t, err)
	assert.Nil(t, resp, "noop FetchSchema returns nil")
}

func TestFetchSchema_FailClosed(t *testing.T) {
	reg := &fakeRegistryClient{fetchErr: errors.New("boom")}
	mgr, audit := buildManagerWithFakes(t, pluginmgr.OnErrorFailClosed, reg, nil)
	a := NewHooks(mgr)

	_, err := a.FetchSchema(context.Background(), &registrypb.FetchSchemaRequest{SchemaId: "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
	require.Len(t, audit.Records, 1)
	assert.Equal(t, pluginmgr.OutcomeError, audit.Records[0].Outcome)
}

func TestFetchSchema_FailOpenSwallowsError(t *testing.T) {
	reg := &fakeRegistryClient{fetchErr: errors.New("boom")}
	mgr, _ := buildManagerWithFakes(t, pluginmgr.OnErrorFailOpen, reg, nil)
	a := NewHooks(mgr)

	resp, err := a.FetchSchema(context.Background(), &registrypb.FetchSchemaRequest{SchemaId: "x"})
	require.NoError(t, err, "fail_open must swallow plugin errors")
	assert.Nil(t, resp)
}

func TestFetchSchema_DeadlineExceededMapsToTimeoutOutcome(t *testing.T) {
	reg := &fakeRegistryClient{fetchDelay: 50 * time.Millisecond}
	cfg := &pluginmgr.Config{
		ConfigVersion: 1,
		Plugins: map[string]pluginmgr.PluginConfig{
			"test": {Binary: "/tmp/ignored", Timeout: 5 * time.Millisecond, OnError: pluginmgr.OnErrorFailClosed},
		},
		Hooks: pluginmgr.HookBindings{FetchSchema: "test"},
	}
	audit := &pluginmgr.RecordingAuditSink{}
	mgr := pluginmgr.NewForTest(cfg, pluginmgr.Options{Audit: audit})
	mgr.InjectClientsForTest("test", reg, nil, pluginmgr.HandleInfo{Name: "test"})

	a := NewHooks(mgr)
	_, err := a.FetchSchema(context.Background(), &registrypb.FetchSchemaRequest{SchemaId: "x"})
	require.Error(t, err)
	require.Len(t, audit.Records, 1)
	assert.Equal(t, pluginmgr.OutcomeTimeout, audit.Records[0].Outcome)
	assert.Equal(t, codes.DeadlineExceeded.String(), audit.Records[0].ErrorCode)
}

func TestFetchSchema_NoHookBoundReturnsNil(t *testing.T) {
	cfg := &pluginmgr.Config{ConfigVersion: 1, Plugins: map[string]pluginmgr.PluginConfig{}}
	mgr := pluginmgr.NewForTest(cfg, pluginmgr.Options{})
	a := NewHooks(mgr)

	resp, err := a.FetchSchema(context.Background(), &registrypb.FetchSchemaRequest{SchemaId: "x"})
	assert.NoError(t, err)
	assert.Nil(t, resp)
}

func TestFetchSchema_PluginBoundButNotRunning(t *testing.T) {
	// Config wires fetch_schema to "test" but no client is injected —
	// simulates a plugin that crashed or never started. Policy applies
	// as if the call errored.
	cfg := &pluginmgr.Config{
		ConfigVersion: 1,
		Plugins: map[string]pluginmgr.PluginConfig{
			"test": {Binary: "/tmp/x", Timeout: time.Second, OnError: pluginmgr.OnErrorFailClosed},
		},
		Hooks: pluginmgr.HookBindings{FetchSchema: "test"},
	}
	mgr := pluginmgr.NewForTest(cfg, pluginmgr.Options{})
	// No InjectClientsForTest — mgr.Registry("test") returns nil.
	a := NewHooks(mgr)

	_, err := a.FetchSchema(context.Background(), &registrypb.FetchSchemaRequest{SchemaId: "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "plugin not running")
}

func TestRegisterConsumerUsage_UnimplementedTreatedAsNoop(t *testing.T) {
	// SDK docs instruct plugin authors to return codes.Unimplemented to
	// opt out of a hook. Core must treat that as success, not as a
	// fail_closed error.
	reg := &fakeRegistryClient{registerErr: status.Error(codes.Unimplemented, "not handled")}
	mgr, _ := buildManagerWithFakes(t, pluginmgr.OnErrorFailClosed, reg, nil)
	a := NewHooks(mgr)

	resp, err := a.RegisterConsumerUsage(context.Background(), &registrypb.RegisterConsumerUsageRequest{ConsumerId: "c", SchemaId: "s"})
	require.NoError(t, err, "Unimplemented must not propagate as error")
	require.NotNil(t, resp)
	assert.True(t, resp.GetAcknowledged())
}

func TestOnBreakingChangeDetected_UnimplementedNoop(t *testing.T) {
	ev := &fakeEventClient{breakErr: status.Error(codes.Unimplemented, "event not handled")}
	mgr, _ := buildManagerWithFakes(t, pluginmgr.OnErrorFailClosed, &fakeRegistryClient{}, ev)
	a := NewHooks(mgr)

	resp, err := a.OnBreakingChangeDetected(context.Background(), &eventspb.BreakingChangeDetectedRequest{SchemaId: "x"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.GetAcknowledged())
}

func TestOnValidationFailed_FailClosedPropagates(t *testing.T) {
	ev := &fakeEventClient{validateErr: errors.New("slack down")}
	mgr, audit := buildManagerWithFakes(t, pluginmgr.OnErrorFailClosed, &fakeRegistryClient{}, ev)
	a := NewHooks(mgr)

	_, err := a.OnValidationFailed(context.Background(), &eventspb.ValidationFailedRequest{SchemaId: "x"})
	require.Error(t, err)
	require.Len(t, audit.Records, 1)
	assert.Equal(t, pluginmgr.AuditKindWrite, audit.Records[0].Kind)
	assert.Equal(t, pluginmgr.OutcomeError, audit.Records[0].Outcome)
}

func TestOnValidationFailed_FailOpenReturnsNoop(t *testing.T) {
	ev := &fakeEventClient{validateErr: errors.New("slack down")}
	mgr, _ := buildManagerWithFakes(t, pluginmgr.OnErrorFailOpen, &fakeRegistryClient{}, ev)
	a := NewHooks(mgr)

	resp, err := a.OnValidationFailed(context.Background(), &eventspb.ValidationFailedRequest{SchemaId: "x"})
	require.NoError(t, err)
	require.NotNil(t, resp)
}
