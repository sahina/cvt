// Command cvt-plugin-slack-events is a CVT EventHandler plugin that
// posts breaking-change and validation-failure events to a Slack
// incoming webhook. It dedups repeated events within a configurable
// window to prevent a broken deploy from spamming a channel.
//
// Config keys:
//
//	webhook_url           required, secret  Slack incoming-webhook URL.
//	channel               optional          Override channel.
//	dedup_window_seconds  optional          Plugin-side dedup window, default 60.
package main

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sahina/cvt/pkg/cvtplugin"
	eventspb "github.com/sahina/cvt/pkg/cvtplugin/pb/events/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type slackPlugin struct {
	mu         sync.RWMutex
	webhookURL string
	channel    string
	dedup      time.Duration

	recentMu sync.Mutex
	recent   map[string]time.Time

	hc *http.Client
}

func newSlack() *slackPlugin {
	return &slackPlugin{
		dedup:  60 * time.Second,
		recent: map[string]time.Time{},
		hc:     &http.Client{Timeout: 5 * time.Second},
	}
}

func (s *slackPlugin) SetConfig(_ context.Context, key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch key {
	case "webhook_url":
		s.webhookURL = value
	case "channel":
		s.channel = value
	case "dedup_window_seconds":
		n, err := strconv.Atoi(value)
		if err != nil || n < 0 {
			return fmt.Errorf("dedup_window_seconds must be a non-negative integer")
		}
		s.dedup = time.Duration(n) * time.Second
	}
	return nil
}

// shouldSend returns true iff this event hash hasn't been sent within
// the dedup window. Updates the recent map atomically when allowing.
func (s *slackPlugin) shouldSend(hash string) bool {
	s.mu.RLock()
	window := s.dedup
	s.mu.RUnlock()
	if window <= 0 {
		return true
	}
	s.recentMu.Lock()
	defer s.recentMu.Unlock()
	now := time.Now()
	if last, ok := s.recent[hash]; ok && now.Sub(last) < window {
		return false
	}
	s.recent[hash] = now
	// Best-effort prune of entries older than 2× window.
	for h, t := range s.recent {
		if now.Sub(t) > 2*window {
			delete(s.recent, h)
		}
	}
	return true
}

func (s *slackPlugin) post(ctx context.Context, text string) error {
	s.mu.RLock()
	url := s.webhookURL
	channel := s.channel
	s.mu.RUnlock()
	if url == "" {
		return status.Error(codes.FailedPrecondition, "webhook_url not configured")
	}
	body := map[string]string{"text": text}
	if channel != "" {
		body["channel"] = channel
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return status.Errorf(codes.Internal, "marshal: %v", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return status.Errorf(codes.Internal, "build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.hc.Do(req)
	if err != nil {
		return status.Errorf(codes.Unavailable, "slack: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode == http.StatusTooManyRequests {
		return status.Error(codes.ResourceExhausted, "slack rate-limited")
	}
	if resp.StatusCode >= 400 {
		return status.Errorf(codes.Unavailable, "slack %d", resp.StatusCode)
	}
	return nil
}

func hashKey(parts ...string) string {
	h := sha1.Sum([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(h[:])
}

func (s *slackPlugin) OnBreakingChangeDetected(ctx context.Context, req *eventspb.BreakingChangeDetectedRequest) (*eventspb.EventResponse, error) {
	key := hashKey("breaking", req.GetSchemaId(), req.GetOldVersion(), req.GetNewVersion())
	if !s.shouldSend(key) {
		return &eventspb.EventResponse{Acknowledged: true}, nil
	}
	changes := make([]string, 0, len(req.GetChanges()))
	for _, c := range req.GetChanges() {
		endpoint := ""
		if c.GetMethod() != "" || c.GetPath() != "" {
			endpoint = fmt.Sprintf(" `%s %s`", c.GetMethod(), c.GetPath())
		}
		changes = append(changes, fmt.Sprintf("• *%s*%s — %s", c.GetKind(), endpoint, c.GetDescription()))
	}
	text := fmt.Sprintf(
		":warning: *Breaking change detected* in `%s` (%s → %s):\n%s",
		req.GetSchemaId(), req.GetOldVersion(), req.GetNewVersion(),
		strings.Join(changes, "\n"),
	)
	if err := s.post(ctx, text); err != nil {
		return nil, err
	}
	return &eventspb.EventResponse{Acknowledged: true}, nil
}

func (s *slackPlugin) OnValidationFailed(ctx context.Context, req *eventspb.ValidationFailedRequest) (*eventspb.EventResponse, error) {
	// Dedup on schema+method+path+first error kind to collapse repeat
	// failures during a broken deploy.
	firstKind := ""
	if len(req.GetErrors()) > 0 {
		firstKind = req.GetErrors()[0].GetKind()
	}
	key := hashKey("failed", req.GetSchemaId(), req.GetMethod(), req.GetPath(), firstKind)
	if !s.shouldSend(key) {
		return &eventspb.EventResponse{Acknowledged: true}, nil
	}
	errs := make([]string, 0, len(req.GetErrors()))
	for _, e := range req.GetErrors() {
		errs = append(errs, fmt.Sprintf("• %s", e.GetDescription()))
	}
	text := fmt.Sprintf(
		":x: *Validation failed* `%s %s` against `%s`:\n%s",
		req.GetMethod(), req.GetPath(), req.GetSchemaId(),
		strings.Join(errs, "\n"),
	)
	if err := s.post(ctx, text); err != nil {
		return nil, err
	}
	return &eventspb.EventResponse{Acknowledged: true}, nil
}

func main() {
	p := newSlack()
	cvtplugin.Serve(
		cvtplugin.PluginInfo{Name: "slack-events", Version: "0.1.0"},
		cvtplugin.WithEventHandler(p),
		cvtplugin.WithConfigReceiver(p),
	)
}
