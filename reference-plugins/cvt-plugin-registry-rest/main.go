// Command cvt-plugin-registry-rest is a CVT RegistryProvider plugin
// that speaks to a generic REST schema registry. It fetches OpenAPI
// specs by ID+version and records consumer usage.
//
// Config keys (all non-secret unless declared in secrets:):
//
//	base_url   required  Registry API base URL.
//	token      optional  Bearer token sent as Authorization header.
//	           secret    Declare in secrets: to keep it out of logs.
//
// Wire endpoints match issue #83's REST contract:
//
//	GET  {base_url}/schemas/{schemaId}/versions/{version}/spec
//	POST {base_url}/schemas/{schemaId}/consumers
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/sahina/cvt/pkg/cvtplugin"
	registrypb "github.com/sahina/cvt/pkg/cvtplugin/pb/registry/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// maxSchemaBytes caps FetchSchema response size. OpenAPI specs above
// this threshold almost certainly indicate a registry misconfiguration
// rather than a legitimate schema. Surfaced as ResourceExhausted.
const maxSchemaBytes int64 = 50 << 20 // 50 MiB

type restRegistry struct {
	mu      sync.RWMutex
	baseURL string
	token   string
	hc      *http.Client
}

func newREST() *restRegistry {
	return &restRegistry{
		hc: &http.Client{Timeout: 10 * time.Second},
	}
}

// SetConfig accepts values delivered by CVT on startup. Unknown keys are
// silently accepted so newer CVT versions can add keys without breaking
// older plugins.
func (r *restRegistry) SetConfig(_ context.Context, key, value string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch key {
	case "base_url":
		r.baseURL = value
	case "token":
		r.token = value
	}
	return nil
}

func (r *restRegistry) snapshot() (string, string) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.baseURL, r.token
}

func (r *restRegistry) FetchSchema(ctx context.Context, req *registrypb.FetchSchemaRequest) (*registrypb.FetchSchemaResponse, error) {
	base, token := r.snapshot()
	if base == "" {
		return nil, status.Error(codes.FailedPrecondition, "base_url not configured")
	}
	if req.GetSchemaId() == "" {
		return nil, status.Error(codes.InvalidArgument, "schema_id required")
	}
	version := req.GetVersion()
	if version == "" {
		version = "latest"
	}
	u := fmt.Sprintf("%s/schemas/%s/versions/%s/spec",
		base, url.PathEscape(req.GetSchemaId()), url.PathEscape(version))

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "build request: %v", err)
	}
	if token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}
	httpReq.Header.Set("Accept", "application/json, application/yaml, */*")

	resp, err := r.hc.Do(httpReq)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "registry: %v", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, status.Errorf(codes.NotFound, "schema %s@%s", req.GetSchemaId(), version)
	case resp.StatusCode >= 500:
		return nil, status.Errorf(codes.Unavailable, "registry %d", resp.StatusCode)
	case resp.StatusCode >= 400:
		return nil, status.Errorf(codes.InvalidArgument, "registry %d", resp.StatusCode)
	}

	// Read up to maxSchemaBytes+1. If we hit the +1 byte, the response
	// overflowed the cap — return a distinct error rather than silently
	// truncating, which would produce confusing downstream parse
	// failures ("unexpected end of JSON").
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSchemaBytes+1))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "read body: %v", err)
	}
	if int64(len(body)) > maxSchemaBytes {
		return nil, status.Errorf(codes.ResourceExhausted,
			"schema %s@%s exceeds %d-byte limit", req.GetSchemaId(), version, maxSchemaBytes)
	}

	// The REST registry returns the resolved version in a header when the
	// request used "latest". Fall back to the requested version, and if
	// the caller asked for "latest" without the server echoing back a
	// concrete version, surface "latest" rather than an empty string so
	// callers have something non-empty to store.
	resolved := resp.Header.Get("X-Schema-Version")
	if resolved == "" {
		if req.GetVersion() == "" {
			resolved = "latest"
		} else {
			resolved = req.GetVersion()
		}
	}
	return &registrypb.FetchSchemaResponse{
		Spec:            body,
		ResolvedVersion: resolved,
	}, nil
}

func (r *restRegistry) RegisterConsumerUsage(ctx context.Context, req *registrypb.RegisterConsumerUsageRequest) (*registrypb.RegisterConsumerUsageResponse, error) {
	base, token := r.snapshot()
	if base == "" {
		return nil, status.Error(codes.FailedPrecondition, "base_url not configured")
	}
	if req.GetSchemaId() == "" || req.GetConsumerId() == "" {
		return nil, status.Error(codes.InvalidArgument, "schema_id and consumer_id required")
	}
	u := fmt.Sprintf("%s/schemas/%s/consumers", base, url.PathEscape(req.GetSchemaId()))

	type endpoint struct {
		Method string `json:"method"`
		Path   string `json:"path"`
	}
	type payload struct {
		ConsumerID    string     `json:"consumerId"`
		SchemaVersion string     `json:"schemaVersion"`
		Environment   string     `json:"environment,omitempty"`
		Endpoints     []endpoint `json:"endpoints"`
	}
	body := payload{
		ConsumerID:    req.GetConsumerId(),
		SchemaVersion: req.GetSchemaVersion(),
		Environment:   req.GetEnvironment(),
	}
	for _, e := range req.GetEndpoints() {
		body.Endpoints = append(body.Endpoints, endpoint{Method: e.GetMethod(), Path: e.GetPath()})
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshal: %v", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(buf))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "build request: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := r.hc.Do(httpReq)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "registry: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	switch {
	case resp.StatusCode >= 500:
		return nil, status.Errorf(codes.Unavailable, "registry %d", resp.StatusCode)
	case resp.StatusCode >= 400:
		return nil, status.Errorf(codes.InvalidArgument, "registry %d", resp.StatusCode)
	}
	return &registrypb.RegisterConsumerUsageResponse{Acknowledged: true}, nil
}

func main() {
	r := newREST()
	cvtplugin.Serve(
		cvtplugin.PluginInfo{Name: "registry-rest", Version: "0.1.0"},
		cvtplugin.WithRegistryProvider(r),
		cvtplugin.WithConfigReceiver(r),
	)
}
