//go:build integration
// +build integration

package pluginmgr_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sahina/cvt/internal/pluginclient"
	"github.com/sahina/cvt/internal/pluginmgr"
	registrypb "github.com/sahina/cvt/pkg/cvtplugin/pb/registry/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// buildReferencePlugin compiles reference-plugins/cvt-plugin-registry-rest
// into the given pluginRoot and returns the binary path. The reference
// plugin lives in its own Go module; this test relies on the go.work
// file including it.
func buildReferencePlugin(t *testing.T, pkg, binName, pluginRoot string) string {
	t.Helper()
	binPath := filepath.Join(pluginRoot, binName)
	cmd := exec.Command("go", "build", "-o", binPath, "./reference-plugins/"+pkg)

	// Compile from the repo root so go.work resolves the nested module.
	root, err := findRepoRoot()
	require.NoError(t, err)
	cmd.Dir = root
	cmd.Env = os.Environ()

	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "build %s failed: %s", pkg, string(out))
	return binPath
}

// findRepoRoot walks up from the current test binary until it finds a
// directory containing go.work. Ensures `go build ./reference-plugins/...`
// resolves through the workspace.
func findRepoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for dir := wd; dir != "/"; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir, nil
		}
	}
	return "", os.ErrNotExist
}

// mockRegistry is an httptest.Server that implements the minimum REST
// contract cvt-plugin-registry-rest speaks: GET schema + POST consumer.
// It records every call for test assertions.
type mockRegistry struct {
	mu           sync.Mutex
	fetchHits    []string
	registerHits []registerCall
}

type registerCall struct {
	SchemaID  string
	ConsumerID string
	Endpoints  []map[string]string
}

func (m *mockRegistry) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		defer m.mu.Unlock()
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/spec") {
			// Path: /schemas/{id}/versions/{version}/spec
			parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
			if len(parts) >= 5 {
				m.fetchHits = append(m.fetchHits, parts[1])
				w.Header().Set("X-Schema-Version", "1.2.3")
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"openapi":"3.0.0","info":{"title":"` + parts[1] + `","version":"1.2.3"},"paths":{}}`))
				return
			}
		}
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/consumers") {
			// Path: /schemas/{id}/consumers
			parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
			schemaID := ""
			if len(parts) >= 3 {
				schemaID = parts[1]
			}
			body, _ := io.ReadAll(r.Body)
			var payload struct {
				ConsumerID string `json:"consumerId"`
				Endpoints  []map[string]string
			}
			_ = json.Unmarshal(body, &payload)
			m.registerHits = append(m.registerHits, registerCall{
				SchemaID:   schemaID,
				ConsumerID: payload.ConsumerID,
				Endpoints:  payload.Endpoints,
			})
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.NotFound(w, r)
	})
}

// TestIntegrationReferencePluginEndToEnd covers plan verification item T5:
// build cvt-plugin-registry-rest, install it via the framework, call
// FetchSchema + RegisterConsumerUsage through the whole stack, and assert
// the mock registry saw both requests.
func TestIntegrationReferencePluginEndToEnd(t *testing.T) {
	// Mock registry first so we know its URL before configuring the plugin.
	mock := &mockRegistry{}
	registryServer := httptest.NewServer(mock.handler())
	defer registryServer.Close()

	tmp := t.TempDir()
	pluginRoot := filepath.Join(tmp, "plugins")
	require.NoError(t, os.MkdirAll(pluginRoot, 0o755))

	binPath := buildReferencePlugin(t, "cvt-plugin-registry-rest", "cvt-plugin-registry-rest", pluginRoot)
	statePath := filepath.Join(pluginRoot, "state.json")
	_, err := pluginmgr.Install(binPath, "registry", pluginRoot, statePath)
	require.NoError(t, err)

	cfg := &pluginmgr.Config{
		ConfigVersion: 1,
		Plugins: map[string]pluginmgr.PluginConfig{
			"registry": {
				Binary:  binPath,
				Timeout: 5 * time.Second,
				OnError: pluginmgr.OnErrorFailClosed,
				Secrets: []string{"token"},
				Config: map[string]string{
					"base_url": registryServer.URL,
					"token":    "test-token-xyz",
				},
			},
		},
		Hooks: pluginmgr.HookBindings{
			FetchSchema:           "registry",
			RegisterConsumerUsage: "registry",
		},
	}
	state, err := pluginmgr.ReadState(statePath)
	require.NoError(t, err)

	audit := &pluginmgr.RecordingAuditSink{}
	mgr := pluginmgr.New(cfg, state, pluginmgr.Options{
		Logger: zaptest.NewLogger(t),
		Audit:  audit,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	require.NoError(t, mgr.Start(ctx))
	defer mgr.Stop()

	hooks := pluginclient.NewHooks(mgr)

	// 1. FetchSchema: plugin calls GET /schemas/pet-api/versions/latest/spec.
	resp, err := hooks.FetchSchema(ctx, &registrypb.FetchSchemaRequest{
		SchemaId:  "pet-api",
		Version:   "latest",
		RequestId: "req-fetch",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Contains(t, string(resp.GetSpec()), `"title":"pet-api"`)
	assert.Equal(t, "1.2.3", resp.GetResolvedVersion())

	// 2. RegisterConsumerUsage: plugin POSTs the consumer record.
	_, err = hooks.RegisterConsumerUsage(ctx, &registrypb.RegisterConsumerUsageRequest{
		ConsumerId:    "order-service",
		SchemaId:      "pet-api",
		SchemaVersion: "1.2.3",
		Environment:   "ci",
		Endpoints: []*registrypb.EndpointUsage{
			{Method: "GET", Path: "/pets/{id}"},
		},
		RequestId: "req-register",
	})
	require.NoError(t, err)

	// Mock registry saw both calls with expected data.
	mock.mu.Lock()
	defer mock.mu.Unlock()
	require.Equal(t, []string{"pet-api"}, mock.fetchHits)
	require.Len(t, mock.registerHits, 1)
	reg := mock.registerHits[0]
	assert.Equal(t, "pet-api", reg.SchemaID)
	assert.Equal(t, "order-service", reg.ConsumerID)
	require.Len(t, reg.Endpoints, 1)
	assert.Equal(t, "GET", reg.Endpoints[0]["method"])
	assert.Equal(t, "/pets/{id}", reg.Endpoints[0]["path"])

	// Audit captured both calls with correct kind + outcome.
	require.Len(t, audit.Records, 2)
	kindsByMethod := map[string]pluginmgr.AuditKind{}
	for _, r := range audit.Records {
		kindsByMethod[r.Method] = r.Kind
		assert.Equal(t, pluginmgr.OutcomeOK, r.Outcome)
		assert.Equal(t, "registry", r.Plugin)
		assert.NotEmpty(t, r.SHA256)
	}
	assert.Equal(t, pluginmgr.AuditKindRead, kindsByMethod["FetchSchema"])
	assert.Equal(t, pluginmgr.AuditKindWrite, kindsByMethod["RegisterConsumerUsage"])

	// Secret redaction: the token must not appear in any audit entry.
	for _, r := range audit.Records {
		bytes, _ := json.Marshal(r)
		assert.NotContainsf(t, string(bytes), "test-token-xyz",
			"secret token leaked into audit record: %s", string(bytes))
	}
}
