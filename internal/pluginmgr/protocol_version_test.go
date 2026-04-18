package pluginmgr

import (
	"context"
	"testing"

	"github.com/sahina/cvt/pkg/cvtplugin"
	handshakepb "github.com/sahina/cvt/pkg/cvtplugin/pb/handshake/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

// fakeHandshakeClient implements handshakepb.PluginHandshakeClient for
// testing the version-check logic without a subprocess. Production
// code path: startOne → hClient.Info → protocol-version compare → reject
// mismatched plugin.
type fakeHandshakeClient struct {
	info *handshakepb.InfoResponse
}

func (f *fakeHandshakeClient) Info(_ context.Context, _ *handshakepb.InfoRequest, _ ...grpc.CallOption) (*handshakepb.InfoResponse, error) {
	return f.info, nil
}
func (f *fakeHandshakeClient) Health(_ context.Context, _ *handshakepb.HealthRequest, _ ...grpc.CallOption) (*handshakepb.HealthResponse, error) {
	return &handshakepb.HealthResponse{Status: handshakepb.HealthResponse_SERVING}, nil
}
func (f *fakeHandshakeClient) SetConfig(_ context.Context, _ *handshakepb.SetConfigRequest, _ ...grpc.CallOption) (*handshakepb.SetConfigResponse, error) {
	return &handshakepb.SetConfigResponse{}, nil
}

// checkProtocolVersion reproduces the guard from manager.startOne so we
// can unit-test the comparison without forking a real plugin.
func checkProtocolVersion(c handshakepb.PluginHandshakeClient) error {
	info, err := c.Info(context.Background(), &handshakepb.InfoRequest{})
	if err != nil {
		return err
	}
	if info.GetProtocolVersion() != cvtplugin.ProtocolVersion {
		return &protocolMismatchError{got: info.GetProtocolVersion(), want: cvtplugin.ProtocolVersion}
	}
	return nil
}

type protocolMismatchError struct{ got, want uint32 }

func (e *protocolMismatchError) Error() string { return "protocol version mismatch" }

func TestProtocolVersion_MatchOK(t *testing.T) {
	c := &fakeHandshakeClient{info: &handshakepb.InfoResponse{
		Name:            "echo",
		Version:         "1.0.0",
		ProtocolVersion: cvtplugin.ProtocolVersion,
		Services:        []string{cvtplugin.ServiceRegistryV1},
	}}
	require.NoError(t, checkProtocolVersion(c))
}

func TestProtocolVersion_MismatchRejected(t *testing.T) {
	c := &fakeHandshakeClient{info: &handshakepb.InfoResponse{
		Name:            "echo",
		ProtocolVersion: cvtplugin.ProtocolVersion + 1,
	}}
	err := checkProtocolVersion(c)
	require.Error(t, err)
	var mme *protocolMismatchError
	if assert.ErrorAs(t, err, &mme) {
		assert.Equal(t, cvtplugin.ProtocolVersion+1, mme.got)
		assert.Equal(t, cvtplugin.ProtocolVersion, mme.want)
	}
}
