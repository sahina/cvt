package plugintest_test

import (
	"context"
	"errors"
	"testing"

	"github.com/sahina/cvt/pkg/cvtplugin"
	eventspb "github.com/sahina/cvt/pkg/cvtplugin/pb/events/v1"
	registrypb "github.com/sahina/cvt/pkg/cvtplugin/pb/registry/v1"
	"github.com/sahina/cvt/pkg/cvtplugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRegistry struct {
	got      map[string]string
	fetchErr error
}

func (r *fakeRegistry) FetchSchema(_ context.Context, req *registrypb.FetchSchemaRequest) (*registrypb.FetchSchemaResponse, error) {
	if r.fetchErr != nil {
		return nil, r.fetchErr
	}
	return &registrypb.FetchSchemaResponse{
		Spec:            []byte("openapi: 3.0.0"),
		ResolvedVersion: "1.0.0",
	}, nil
}

func (r *fakeRegistry) RegisterConsumerUsage(_ context.Context, req *registrypb.RegisterConsumerUsageRequest) (*registrypb.RegisterConsumerUsageResponse, error) {
	return &registrypb.RegisterConsumerUsageResponse{Acknowledged: true}, nil
}

// fakeRegistry also implements ConfigReceiver.
func (r *fakeRegistry) SetConfig(_ context.Context, key, value string) error {
	if r.got == nil {
		r.got = map[string]string{}
	}
	r.got[key] = value
	return nil
}

type fakeEvents struct {
	seen []string
}

func (e *fakeEvents) OnBreakingChangeDetected(_ context.Context, req *eventspb.BreakingChangeDetectedRequest) (*eventspb.EventResponse, error) {
	e.seen = append(e.seen, "breaking:"+req.GetSchemaId())
	return &eventspb.EventResponse{Acknowledged: true}, nil
}

func (e *fakeEvents) OnValidationFailed(_ context.Context, req *eventspb.ValidationFailedRequest) (*eventspb.EventResponse, error) {
	e.seen = append(e.seen, "failed:"+req.GetSchemaId())
	return &eventspb.EventResponse{Acknowledged: true}, nil
}

func TestHarnessRegistryRoundTrip(t *testing.T) {
	reg := &fakeRegistry{}
	h := plugintest.New().WithRegistry(reg)

	resp, err := h.FetchSchema(context.Background(), &registrypb.FetchSchemaRequest{SchemaId: "pet-api"})
	require.NoError(t, err)
	assert.Equal(t, []byte("openapi: 3.0.0"), resp.GetSpec())
	assert.Equal(t, "1.0.0", resp.GetResolvedVersion())
}

func TestHarnessRegistryError(t *testing.T) {
	reg := &fakeRegistry{fetchErr: errors.New("boom")}
	h := plugintest.New().WithRegistry(reg)

	_, err := h.FetchSchema(context.Background(), &registrypb.FetchSchemaRequest{})
	assert.EqualError(t, err, "boom")
}

func TestHarnessConfigReachesReceiverAndRecords(t *testing.T) {
	reg := &fakeRegistry{}
	h := plugintest.New().WithRegistry(reg)

	err := h.SetConfig(context.Background(), "token", "s3cret")
	require.NoError(t, err)
	assert.Equal(t, "s3cret", reg.got["token"])
	assert.Equal(t, "s3cret", h.Config["token"])
}

func TestHarnessEvents(t *testing.T) {
	ev := &fakeEvents{}
	h := plugintest.New().WithEvents(ev)

	_, err := h.OnBreakingChangeDetected(context.Background(), &eventspb.BreakingChangeDetectedRequest{SchemaId: "x"})
	require.NoError(t, err)
	_, err = h.OnValidationFailed(context.Background(), &eventspb.ValidationFailedRequest{SchemaId: "y"})
	require.NoError(t, err)

	assert.Equal(t, []string{"breaking:x", "failed:y"}, ev.seen)
}

// Ensure the SDK's exported plugin keys stay stable — external consumers
// (reference plugins, internal/pluginmgr) depend on these constants.
func TestExportedPluginKeysStable(t *testing.T) {
	assert.Equal(t, "handshake", cvtplugin.PluginKeyHandshake)
	assert.Equal(t, "registry", cvtplugin.PluginKeyRegistry)
	assert.Equal(t, "events", cvtplugin.PluginKeyEvents)
}

func TestHandshakeCookieStable(t *testing.T) {
	assert.Equal(t, "CVT_PLUGIN_MAGIC_COOKIE", cvtplugin.Handshake.MagicCookieKey)
	assert.Equal(t, "cvt-plugin-v1", cvtplugin.Handshake.MagicCookieValue)
}
