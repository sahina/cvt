package cvtplugin

import (
	"context"
	"errors"
	"testing"

	handshakepb "github.com/sahina/cvt/pkg/cvtplugin/pb/handshake/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandshakeInfo(t *testing.T) {
	svc := &handshakeService{
		info:     PluginInfo{Name: "my-plugin", Version: "1.2.3"},
		services: []string{ServiceRegistryV1},
		healthy:  true,
	}

	resp, err := svc.Info(context.Background(), &handshakepb.InfoRequest{})
	require.NoError(t, err)
	assert.Equal(t, "my-plugin", resp.GetName())
	assert.Equal(t, "1.2.3", resp.GetVersion())
	assert.Equal(t, []string{ServiceRegistryV1}, resp.GetServices())
	assert.Equal(t, ProtocolVersion, resp.GetProtocolVersion())
}

func TestHandshakeHealthServing(t *testing.T) {
	svc := &handshakeService{healthy: true}
	resp, err := svc.Health(context.Background(), &handshakepb.HealthRequest{})
	require.NoError(t, err)
	assert.Equal(t, handshakepb.HealthResponse_SERVING, resp.GetStatus())
}

func TestHandshakeHealthNotServing(t *testing.T) {
	svc := &handshakeService{healthy: false}
	resp, err := svc.Health(context.Background(), &handshakepb.HealthRequest{})
	require.NoError(t, err)
	assert.Equal(t, handshakepb.HealthResponse_NOT_SERVING, resp.GetStatus())
}

type fakeReceiver struct {
	got  map[string]string
	fail bool
}

func (r *fakeReceiver) SetConfig(_ context.Context, key, value string) error {
	if r.fail {
		return errors.New("receiver rejects")
	}
	if r.got == nil {
		r.got = map[string]string{}
	}
	r.got[key] = value
	return nil
}

func TestHandshakeSetConfigWithReceiver(t *testing.T) {
	rcv := &fakeReceiver{}
	svc := &handshakeService{receiver: rcv}

	_, err := svc.SetConfig(context.Background(), &handshakepb.SetConfigRequest{Key: "token", Value: "s3cret"})
	require.NoError(t, err)
	assert.Equal(t, "s3cret", rcv.got["token"])
}

func TestHandshakeSetConfigWithoutReceiver(t *testing.T) {
	svc := &handshakeService{receiver: nil}
	_, err := svc.SetConfig(context.Background(), &handshakepb.SetConfigRequest{Key: "x", Value: "y"})
	require.NoError(t, err)
}

func TestHandshakeSetConfigReceiverError(t *testing.T) {
	rcv := &fakeReceiver{fail: true}
	svc := &handshakeService{receiver: rcv}
	_, err := svc.SetConfig(context.Background(), &handshakepb.SetConfigRequest{Key: "x", Value: "y"})
	assert.Error(t, err)
}
