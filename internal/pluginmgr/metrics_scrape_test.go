//go:build integration
// +build integration

package pluginmgr_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sahina/cvt/internal/pluginclient"
	"github.com/sahina/cvt/internal/pluginmgr"
	registrypb "github.com/sahina/cvt/pkg/cvtplugin/pb/registry/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// TestIntegrationMetricsScrape covers plan verification item T6: start
// the framework with a plugin configured, trigger an RPC, scrape the
// /metrics endpoint, and confirm every cvt_plugin_* series appears with
// expected labels.
func TestIntegrationMetricsScrape(t *testing.T) {
	tmp := t.TempDir()
	pluginRoot := filepath.Join(tmp, "plugins")
	require.NoError(t, os.MkdirAll(pluginRoot, 0o755))
	binPath := buildEchoPlugin(t, pluginRoot)
	statePath := filepath.Join(pluginRoot, "state.json")
	_, err := pluginmgr.Install(binPath, "echo", pluginRoot, statePath)
	require.NoError(t, err)

	// Build metrics against a fresh registry so the test owns its scrape
	// surface; this also avoids collisions with any default-registerer
	// state from other tests in the same binary.
	reg := prometheus.NewRegistry()
	m := pluginmgr.NewMetrics(reg)

	cfg := &pluginmgr.Config{
		ConfigVersion: 1,
		Plugins: map[string]pluginmgr.PluginConfig{
			"echo": {
				Binary:  binPath,
				Timeout: 5 * time.Second,
				OnError: pluginmgr.OnErrorFailClosed,
			},
		},
		Hooks: pluginmgr.HookBindings{FetchSchema: "echo"},
	}
	state, err := pluginmgr.ReadState(statePath)
	require.NoError(t, err)

	mgr := pluginmgr.New(cfg, state, pluginmgr.Options{
		Logger:  zaptest.NewLogger(t),
		Metrics: m,
		Audit:   pluginmgr.NullAuditSink{},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	require.NoError(t, mgr.Start(ctx))
	defer mgr.Stop()

	// Trigger an RPC so every series has at least one observation/sample.
	hooks := pluginclient.NewHooks(mgr)
	_, err = hooks.FetchSchema(context.Background(), &registrypb.FetchSchemaRequest{
		SchemaId: "pet-api", RequestId: "req-metrics",
	})
	require.NoError(t, err)

	// Scrape /metrics via an httptest server backed by the custom registry.
	ts := httptest.NewServer(promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	defer ts.Close()
	resp, err := http.Get(ts.URL + "/metrics")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	text := string(body)

	// Series with observations must appear in the scrape output with the
	// expected labels.
	for _, needle := range []string{
		`# TYPE cvt_plugin_call_duration_seconds histogram`,
		`cvt_plugin_call_duration_seconds_count{method="FetchSchema",plugin="echo",service="registry.v1"} 1`,
		`# TYPE cvt_plugin_up gauge`,
		`cvt_plugin_up{plugin="echo"} 1`,
		`# TYPE cvt_plugin_info gauge`,
		`cvt_plugin_info{plugin="echo",sha256="`,
		`,version="echo/0.0.1"} 1`,
	} {
		assert.Truef(t, strings.Contains(text, needle), "metric line missing: %s\n--- full body ---\n%s", needle, text)
	}

	// Trigger an error so cvt_plugin_call_errors_total emits at least one
	// sample. We call an Events method on a plugin that doesn't declare
	// events.v1 — this produces a gRPC error via the nil client branch.
	// Well, in this test the echo plugin DOES declare events.v1, so we
	// simulate an error differently: explicitly touch the errors counter
	// at a known label combination. This is fine — the test's purpose is
	// to verify the scrape surface, not to exercise an organic error path
	// (other tests cover on_error behavior).
	m.CallErrors.WithLabelValues("echo", "registry.v1", "FetchSchema", "DeadlineExceeded").Add(0)
	m.Restarts.WithLabelValues("echo").Add(0)

	// Re-scrape and confirm the counter families now surface.
	resp2, err := http.Get(ts.URL + "/metrics")
	require.NoError(t, err)
	defer resp2.Body.Close()
	body2, err := io.ReadAll(resp2.Body)
	require.NoError(t, err)
	text2 := string(body2)
	for _, needle := range []string{
		`# TYPE cvt_plugin_call_errors_total counter`,
		`# TYPE cvt_plugin_restarts_total counter`,
	} {
		assert.Truef(t, strings.Contains(text2, needle), "metric family missing after touch: %s", needle)
	}
}
