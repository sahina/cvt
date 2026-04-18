package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	registrypb "github.com/sahina/cvt/pkg/cvtplugin/pb/registry/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func configured(t *testing.T, baseURL, token string) *restRegistry {
	t.Helper()
	r := newREST()
	require.NoError(t, r.SetConfig(context.Background(), "base_url", baseURL))
	if token != "" {
		require.NoError(t, r.SetConfig(context.Background(), "token", token))
	}
	return r
}

func TestFetchSchema_Happy(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		assert.Equal(t, "Bearer s3cret", req.Header.Get("Authorization"))
		w.Header().Set("X-Schema-Version", "1.2.3")
		_, _ = w.Write([]byte(`{"openapi":"3.0.0"}`))
	}))
	defer ts.Close()

	r := configured(t, ts.URL, "s3cret")
	resp, err := r.FetchSchema(context.Background(), &registrypb.FetchSchemaRequest{SchemaId: "pet-api", Version: "latest"})
	require.NoError(t, err)
	assert.Contains(t, string(resp.GetSpec()), `"openapi":"3.0.0"`)
	assert.Equal(t, "1.2.3", resp.GetResolvedVersion())
}

func TestFetchSchema_NoTokenOmitsHeader(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		assert.Empty(t, req.Header.Get("Authorization"), "token unset → no Authorization header")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	r := configured(t, ts.URL, "")
	_, err := r.FetchSchema(context.Background(), &registrypb.FetchSchemaRequest{SchemaId: "x"})
	require.NoError(t, err)
}

func TestFetchSchema_StatusCodeMapping(t *testing.T) {
	cases := []struct {
		status   int
		wantCode codes.Code
	}{
		{http.StatusNotFound, codes.NotFound},
		{http.StatusBadRequest, codes.InvalidArgument},
		{http.StatusUnauthorized, codes.InvalidArgument},
		{http.StatusInternalServerError, codes.Unavailable},
		{http.StatusBadGateway, codes.Unavailable},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
			}))
			defer ts.Close()

			r := configured(t, ts.URL, "")
			_, err := r.FetchSchema(context.Background(), &registrypb.FetchSchemaRequest{SchemaId: "x"})
			require.Error(t, err)
			assert.Equal(t, tc.wantCode, status.Code(err), "status %d → gRPC code", tc.status)
		})
	}
}

func TestFetchSchema_OversizedBodyResourceExhausted(t *testing.T) {
	// Server returns maxSchemaBytes+1 bytes. Plugin must reject rather
	// than silently truncate.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		blob := make([]byte, maxSchemaBytes+1)
		for i := range blob {
			blob[i] = 'a'
		}
		_, _ = w.Write(blob)
	}))
	defer ts.Close()

	r := configured(t, ts.URL, "")
	_, err := r.FetchSchema(context.Background(), &registrypb.FetchSchemaRequest{SchemaId: "x"})
	require.Error(t, err)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))
	assert.Contains(t, err.Error(), "limit")
}

func TestFetchSchema_BaseURLRequired(t *testing.T) {
	r := newREST()
	_, err := r.FetchSchema(context.Background(), &registrypb.FetchSchemaRequest{SchemaId: "x"})
	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestFetchSchema_SchemaIDRequired(t *testing.T) {
	r := configured(t, "http://example.com", "")
	_, err := r.FetchSchema(context.Background(), &registrypb.FetchSchemaRequest{SchemaId: ""})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestFetchSchema_DefaultsVersionToLatest(t *testing.T) {
	var seenPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		seenPath = req.URL.Path
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	r := configured(t, ts.URL, "")
	_, err := r.FetchSchema(context.Background(), &registrypb.FetchSchemaRequest{SchemaId: "pet"})
	require.NoError(t, err)
	assert.Contains(t, seenPath, "/versions/latest/spec")
}

func TestRegisterConsumerUsage_Happy(t *testing.T) {
	var receivedBody []byte
	var receivedPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		receivedBody, _ = io.ReadAll(req.Body)
		receivedPath = req.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	r := configured(t, ts.URL, "tok")
	resp, err := r.RegisterConsumerUsage(context.Background(), &registrypb.RegisterConsumerUsageRequest{
		ConsumerId:    "order-service",
		SchemaId:      "pet-api",
		SchemaVersion: "1.2.3",
		Environment:   "ci",
		Endpoints: []*registrypb.EndpointUsage{
			{Method: "GET", Path: "/pets/{id}"},
		},
	})
	require.NoError(t, err)
	assert.True(t, resp.GetAcknowledged())
	assert.Equal(t, "/schemas/pet-api/consumers", receivedPath)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(receivedBody, &body))
	assert.Equal(t, "order-service", body["consumerId"])
	assert.Equal(t, "1.2.3", body["schemaVersion"])
	assert.Equal(t, "ci", body["environment"])
}

func TestRegisterConsumerUsage_RequiresConsumerAndSchema(t *testing.T) {
	r := configured(t, "http://example.com", "")
	_, err := r.RegisterConsumerUsage(context.Background(), &registrypb.RegisterConsumerUsageRequest{SchemaId: "x"})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestRegisterConsumerUsage_ServerErrorMapsToUnavailable(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer ts.Close()

	r := configured(t, ts.URL, "")
	_, err := r.RegisterConsumerUsage(context.Background(), &registrypb.RegisterConsumerUsageRequest{
		ConsumerId: "c", SchemaId: "s",
	})
	require.Error(t, err)
	assert.Equal(t, codes.Unavailable, status.Code(err))
}

func TestSetConfigIgnoresUnknownKeys(t *testing.T) {
	r := newREST()
	// Unknown keys must be accepted silently — newer CVT versions may
	// add keys older plugin versions don't recognize.
	require.NoError(t, r.SetConfig(context.Background(), "future_option", "v"))
}

func TestSchemaIDURLEscaped(t *testing.T) {
	var seenRawPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// EscapedPath() preserves percent-encoding; req.URL.Path is
		// always decoded. We need to verify what was actually sent on
		// the wire, not the server-side decoded form.
		seenRawPath = req.URL.EscapedPath()
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	r := configured(t, ts.URL, "")
	_, err := r.FetchSchema(context.Background(), &registrypb.FetchSchemaRequest{SchemaId: "foo/bar"})
	require.NoError(t, err)
	// The {id} path segment must be percent-encoded so "foo/bar" stays
	// inside that segment rather than adding a new path component.
	assert.Contains(t, seenRawPath, "foo%2Fbar", "schema_id must be URL-escaped: got %s", seenRawPath)
}
