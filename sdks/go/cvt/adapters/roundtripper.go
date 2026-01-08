package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/cvt/cvt-sdk/go/cvt"
)

// RoundTripperConfig configures the ValidatingRoundTripper.
type RoundTripperConfig struct {
	// Validator is the CVT validator instance (required).
	// Accepts any type implementing the Validator interface.
	Validator Validator

	// Transport is the underlying http.RoundTripper (default: http.DefaultTransport).
	Transport http.RoundTripper

	// AutoValidate enables automatic validation (default: true).
	AutoValidate bool

	// OnValidationFailure is called when validation fails.
	// If it returns an error, that error is returned from RoundTrip.
	// If nil, validation failures are silently recorded.
	OnValidationFailure func(result *cvt.ValidationResult, req *http.Request, resp *http.Response) error

	// IncludePaths filters requests to only validate matching paths.
	IncludePaths []PathFilter

	// ExcludePaths filters requests to exclude matching paths.
	ExcludePaths []PathFilter
}

// ValidatingRoundTripper is an http.RoundTripper that validates HTTP traffic.
//
// Example:
//
//	validator, _ := cvt.NewValidator("")
//	validator.RegisterSchema(ctx, "api", "./openapi.json")
//
//	rt := adapters.NewValidatingRoundTripper(adapters.RoundTripperConfig{
//	    Validator:    validator,
//	    AutoValidate: true,
//	})
//
//	client := &http.Client{Transport: rt}
//	resp, err := client.Get("http://api.test/pet/1")
//
//	// Check interactions
//	interactions := rt.GetInteractions()
type ValidatingRoundTripper struct {
	config       RoundTripperConfig
	interactions []CapturedInteraction
	mu           sync.Mutex
}

// NewValidatingRoundTripper creates a new ValidatingRoundTripper.
func NewValidatingRoundTripper(config RoundTripperConfig) *ValidatingRoundTripper {
	if config.Validator == nil {
		panic("cvt: Validator is required")
	}
	if config.Transport == nil {
		config.Transport = http.DefaultTransport
	}

	return &ValidatingRoundTripper{
		config:       config,
		interactions: make([]CapturedInteraction, 0),
	}
}

// RoundTrip implements http.RoundTripper.
func (rt *ValidatingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Capture request body before sending
	var reqBody []byte
	if req.Body != nil {
		var err error
		reqBody, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}
		req.Body = io.NopCloser(bytes.NewReader(reqBody))
	}

	// Execute the actual request
	resp, err := rt.config.Transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// Check if we should validate this path
	path := req.URL.Path
	if req.URL.RawQuery != "" {
		path += "?" + req.URL.RawQuery
	}

	if !shouldValidatePath(path, rt.config.IncludePaths, rt.config.ExcludePaths) {
		return resp, nil
	}

	// Capture response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	resp.Body = io.NopCloser(bytes.NewReader(respBody))

	// Build validation objects
	validationReq := rt.extractRequest(req, reqBody)
	validationResp := rt.extractResponse(resp, respBody)

	interaction := CapturedInteraction{
		Request:   validationReq,
		Response:  validationResp,
		Timestamp: time.Now(),
	}

	// Validate if auto-validate is enabled (default is true when not explicitly set to false)
	if rt.config.AutoValidate || rt.config.OnValidationFailure != nil {
		ctx := req.Context()
		result, validateErr := rt.config.Validator.Validate(ctx, validationReq, validationResp)
		if validateErr == nil {
			interaction.ValidationResult = result
			if !result.Valid && rt.config.OnValidationFailure != nil {
				if failErr := rt.config.OnValidationFailure(result, req, resp); failErr != nil {
					rt.addInteraction(interaction)
					return nil, failErr
				}
			}
		}
	}

	rt.addInteraction(interaction)
	return resp, nil
}

func (rt *ValidatingRoundTripper) addInteraction(interaction CapturedInteraction) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.interactions = append(rt.interactions, interaction)
}

func (rt *ValidatingRoundTripper) extractRequest(req *http.Request, body []byte) cvt.ValidationRequest {
	headers := make(map[string]string)
	for k, v := range req.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	var bodyData any
	if len(body) > 0 {
		_ = json.Unmarshal(body, &bodyData)
	}

	path := req.URL.Path
	if req.URL.RawQuery != "" {
		path += "?" + req.URL.RawQuery
	}

	return cvt.ValidationRequest{
		Method:  req.Method,
		Path:    path,
		Headers: headers,
		Body:    bodyData,
	}
}

func (rt *ValidatingRoundTripper) extractResponse(resp *http.Response, body []byte) cvt.ValidationResponse {
	headers := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	var bodyData any
	if len(body) > 0 {
		_ = json.Unmarshal(body, &bodyData)
	}

	return cvt.ValidationResponse{
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Body:       bodyData,
	}
}

// GetInteractions returns all captured interactions.
func (rt *ValidatingRoundTripper) GetInteractions() []CapturedInteraction {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	result := make([]CapturedInteraction, len(rt.interactions))
	copy(result, rt.interactions)
	return result
}

// ClearInteractions clears all captured interactions.
func (rt *ValidatingRoundTripper) ClearInteractions() {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.interactions = rt.interactions[:0]
}

// ValidateInteraction manually validates a captured interaction.
func (rt *ValidatingRoundTripper) ValidateInteraction(ctx context.Context, interaction CapturedInteraction) (*cvt.ValidationResult, error) {
	return rt.config.Validator.Validate(ctx, interaction.Request, interaction.Response)
}
