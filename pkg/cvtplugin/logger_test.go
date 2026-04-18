package cvtplugin

import (
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func newObservedLogger(t *testing.T) (*zap.Logger, *observer.ObservedLogs) {
	t.Helper()
	core, recorded := observer.New(zap.DebugLevel)
	return zap.New(core), recorded
}

func TestHCLogForwardsToZapWithPluginField(t *testing.T) {
	z, recorded := newObservedLogger(t)
	l := NewHCLogFromZap(z, "my-plugin")

	l.Info("fetched", "schema_id", "pet-api", "duration_ms", 42)

	entries := recorded.TakeAll()
	require.Len(t, entries, 1)
	e := entries[0]
	assert.Equal(t, "fetched", e.Message)
	fields := e.ContextMap()
	assert.Equal(t, "my-plugin", fields["plugin"])
	assert.Equal(t, "pet-api", fields["schema_id"])
	assert.Equal(t, int64(42), fields["duration_ms"])
}

func TestHCLogLevelMapping(t *testing.T) {
	z, recorded := newObservedLogger(t)
	l := NewHCLogFromZap(z, "p")

	l.Debug("d")
	l.Info("i")
	l.Warn("w")
	l.Error("e")

	entries := recorded.TakeAll()
	require.Len(t, entries, 4)
	assert.Equal(t, zap.DebugLevel, entries[0].Level)
	assert.Equal(t, zap.InfoLevel, entries[1].Level)
	assert.Equal(t, zap.WarnLevel, entries[2].Level)
	assert.Equal(t, zap.ErrorLevel, entries[3].Level)
}

func TestHCLogWithAddsFields(t *testing.T) {
	z, recorded := newObservedLogger(t)
	l := NewHCLogFromZap(z, "p").With("request_id", "abc123")
	l.Info("msg")

	entries := recorded.TakeAll()
	require.Len(t, entries, 1)
	fields := entries[0].ContextMap()
	assert.Equal(t, "abc123", fields["request_id"])
}

func TestHCLogNamedAppendsName(t *testing.T) {
	z, _ := newObservedLogger(t)
	l := NewHCLogFromZap(z, "p").Named("sub")
	assert.Equal(t, "p.sub", l.Name())
}

func TestHCLogGetLevel(t *testing.T) {
	z, _ := newObservedLogger(t)
	l := NewHCLogFromZap(z, "p")
	// Observer core enables DebugLevel per newObservedLogger.
	assert.Equal(t, hclog.Debug, l.GetLevel())
}

func TestHCLogNilZapIsNopSafe(t *testing.T) {
	l := NewHCLogFromZap(nil, "p")
	// Must not panic.
	l.Info("msg", "k", "v")
}
