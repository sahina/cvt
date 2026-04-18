package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	eventspb "github.com/sahina/cvt/pkg/cvtplugin/pb/events/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func configured(t *testing.T, webhookURL string, dedup string) *slackPlugin {
	t.Helper()
	p := newSlack()
	require.NoError(t, p.SetConfig(context.Background(), "webhook_url", webhookURL))
	if dedup != "" {
		require.NoError(t, p.SetConfig(context.Background(), "dedup_window_seconds", dedup))
	}
	return p
}

func TestOnBreakingChangeDetected_PostsToWebhook(t *testing.T) {
	var posted int32
	var lastBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		atomic.AddInt32(&posted, 1)
		lastBody, _ = io.ReadAll(req.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	p := configured(t, ts.URL, "")
	resp, err := p.OnBreakingChangeDetected(context.Background(), &eventspb.BreakingChangeDetectedRequest{
		SchemaId:   "pet-api",
		OldVersion: "1.0.0",
		NewVersion: "2.0.0",
		Changes: []*eventspb.BreakingChange{
			{Kind: "removed_endpoint", Path: "/pets/{id}", Method: "DELETE", Description: "Endpoint removed"},
		},
	})
	require.NoError(t, err)
	assert.True(t, resp.GetAcknowledged())
	assert.Equal(t, int32(1), atomic.LoadInt32(&posted))

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(lastBody, &body))
	text, _ := body["text"].(string)
	assert.Contains(t, text, "pet-api")
	assert.Contains(t, text, "removed_endpoint")
}

func TestDedup_SecondCallInWindowDoesNotPost(t *testing.T) {
	var posted int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&posted, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	p := configured(t, ts.URL, "60") // 60s window
	req := &eventspb.BreakingChangeDetectedRequest{SchemaId: "s", OldVersion: "1", NewVersion: "2"}

	_, err := p.OnBreakingChangeDetected(context.Background(), req)
	require.NoError(t, err)
	_, err = p.OnBreakingChangeDetected(context.Background(), req)
	require.NoError(t, err)

	assert.Equal(t, int32(1), atomic.LoadInt32(&posted), "second identical event must be dedup'd")
}

func TestDedup_ZeroWindowDisablesDedup(t *testing.T) {
	var posted int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&posted, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	p := configured(t, ts.URL, "0")
	req := &eventspb.BreakingChangeDetectedRequest{SchemaId: "s"}
	for i := 0; i < 3; i++ {
		_, err := p.OnBreakingChangeDetected(context.Background(), req)
		require.NoError(t, err)
	}
	assert.Equal(t, int32(3), atomic.LoadInt32(&posted), "dedup_window=0 must forward every event")
}

func TestDedup_DifferentEventsNotDeduped(t *testing.T) {
	var posted int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&posted, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	p := configured(t, ts.URL, "60")
	_, err := p.OnBreakingChangeDetected(context.Background(), &eventspb.BreakingChangeDetectedRequest{SchemaId: "a"})
	require.NoError(t, err)
	_, err = p.OnBreakingChangeDetected(context.Background(), &eventspb.BreakingChangeDetectedRequest{SchemaId: "b"})
	require.NoError(t, err)
	assert.Equal(t, int32(2), atomic.LoadInt32(&posted), "distinct events must not dedup")
}

func TestDedup_WindowExpiryResends(t *testing.T) {
	var posted int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&posted, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	p := configured(t, ts.URL, "")
	// Override dedup window to something tiny for test speed.
	p.mu.Lock()
	p.dedup = 20 * time.Millisecond
	p.mu.Unlock()

	req := &eventspb.BreakingChangeDetectedRequest{SchemaId: "s"}
	_, err := p.OnBreakingChangeDetected(context.Background(), req)
	require.NoError(t, err)

	time.Sleep(40 * time.Millisecond)

	_, err = p.OnBreakingChangeDetected(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, int32(2), atomic.LoadInt32(&posted), "event after window expiry must re-send")
}

func TestPost_MissingWebhookFailedPrecondition(t *testing.T) {
	p := newSlack()
	_, err := p.OnBreakingChangeDetected(context.Background(), &eventspb.BreakingChangeDetectedRequest{SchemaId: "s"})
	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestPost_RateLimitMapsToResourceExhausted(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	p := configured(t, ts.URL, "")
	_, err := p.OnBreakingChangeDetected(context.Background(), &eventspb.BreakingChangeDetectedRequest{SchemaId: "s"})
	require.Error(t, err)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))
}

func TestPost_5xxMapsToUnavailable(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	p := configured(t, ts.URL, "")
	_, err := p.OnBreakingChangeDetected(context.Background(), &eventspb.BreakingChangeDetectedRequest{SchemaId: "s"})
	require.Error(t, err)
	assert.Equal(t, codes.Unavailable, status.Code(err))
}

func TestSetConfig_DedupWindowValidation(t *testing.T) {
	p := newSlack()
	assert.Error(t, p.SetConfig(context.Background(), "dedup_window_seconds", "abc"))
	assert.Error(t, p.SetConfig(context.Background(), "dedup_window_seconds", "-5"))
	assert.NoError(t, p.SetConfig(context.Background(), "dedup_window_seconds", "0"))
	assert.NoError(t, p.SetConfig(context.Background(), "dedup_window_seconds", "120"))
}

func TestSetConfig_ChannelOverride(t *testing.T) {
	var body map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		raw, _ := io.ReadAll(req.Body)
		_ = json.Unmarshal(raw, &body)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	p := configured(t, ts.URL, "")
	require.NoError(t, p.SetConfig(context.Background(), "channel", "#ops-alerts"))
	_, err := p.OnValidationFailed(context.Background(), &eventspb.ValidationFailedRequest{SchemaId: "x"})
	require.NoError(t, err)
	assert.Equal(t, "#ops-alerts", body["channel"])
}
