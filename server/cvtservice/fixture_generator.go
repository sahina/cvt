// Test fixture generation. Produces request/response examples from an
// OpenAPI schema, delegating shape generation to pkg/cvt.
package cvtservice

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/sahina/cvt/pkg/cvt"
	"github.com/sahina/cvt/server/pb"
	"go.uber.org/zap"
)

// GenerateFixture generates test fixtures from an OpenAPI schema.
// This method creates request/response pairs based on the schema definition,
// useful for testing APIs without making actual HTTP calls.
func (s *ValidatorService) GenerateFixture(ctx context.Context, req *pb.GenerateFixtureRequest) (*pb.GenerateFixtureResponse, error) {
	start := time.Now()
	defer func() {
		grpcRequestDuration.WithLabelValues("GenerateFixture").Observe(time.Since(start).Seconds())
	}()

	Info("Received GenerateFixture request",
		zap.String("schemaId", req.SchemaId),
		zap.String("method", req.Method),
		zap.String("path", req.Path))

	// Validate inputs
	if err := ValidateSchemaID(req.SchemaId); err != nil {
		grpcRequestsTotal.WithLabelValues("GenerateFixture", "failure").Inc()
		return &pb.GenerateFixtureResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid schema ID: %v", err),
		}, nil
	}

	if req.Method == "" || req.Path == "" {
		grpcRequestsTotal.WithLabelValues("GenerateFixture", "failure").Inc()
		return &pb.GenerateFixtureResponse{
			Success: false,
			Message: "Method and path are required",
		}, nil
	}

	// Ensure schema is available in the embedded generator (triggers storage rehydration if needed)
	_, found := s.getSchemaEntry(ctx, req.SchemaId, "")
	if !found {
		grpcRequestsTotal.WithLabelValues("GenerateFixture", "failure").Inc()
		return &pb.GenerateFixtureResponse{
			Success: false,
			Message: fmt.Sprintf("Schema not found: %s", req.SchemaId),
		}, nil
	}

	method := strings.ToUpper(req.Method)
	opts := cvt.GenerateOptions{
		StatusCode:  int(req.StatusCode),
		UseExamples: req.UseExamples,
		ContentType: req.ContentType,
	}
	if opts.ContentType == "" {
		opts.ContentType = "application/json"
	}

	// Generate based on output type, delegating to pkg/cvt
	switch req.OutputType {
	case pb.OutputType_OUTPUT_REQUEST:
		body, err := s.generator.GenerateRequestBody(req.SchemaId, method, req.Path, opts)
		if err != nil {
			grpcRequestsTotal.WithLabelValues("GenerateFixture", "failure").Inc()
			return &pb.GenerateFixtureResponse{
				Success: false,
				Message: fmt.Sprintf("Failed to generate request body: %v", err),
			}, nil
		}
		jsonData, err := json.MarshalIndent(body, "", "  ")
		if err != nil {
			grpcRequestsTotal.WithLabelValues("GenerateFixture", "failure").Inc()
			return &pb.GenerateFixtureResponse{
				Success: false,
				Message: fmt.Sprintf("Failed to marshal request body: %v", err),
			}, nil
		}
		grpcRequestsTotal.WithLabelValues("GenerateFixture", "success").Inc()
		return &pb.GenerateFixtureResponse{
			Success:     true,
			Message:     "Request body generated successfully",
			RequestBody: string(jsonData),
		}, nil

	case pb.OutputType_OUTPUT_RESPONSE:
		resp, err := s.generator.GenerateResponse(req.SchemaId, method, req.Path, opts)
		if err != nil {
			grpcRequestsTotal.WithLabelValues("GenerateFixture", "failure").Inc()
			return &pb.GenerateFixtureResponse{
				Success: false,
				Message: fmt.Sprintf("Failed to generate response: %v", err),
			}, nil
		}
		grpcRequestsTotal.WithLabelValues("GenerateFixture", "success").Inc()
		return &pb.GenerateFixtureResponse{
			Success:  true,
			Message:  "Response generated successfully",
			Response: convertResponse(resp),
		}, nil

	default: // OUTPUT_FIXTURE
		fixture, err := s.generator.GenerateFixture(req.SchemaId, method, req.Path, opts)
		if err != nil {
			grpcRequestsTotal.WithLabelValues("GenerateFixture", "failure").Inc()
			return &pb.GenerateFixtureResponse{
				Success: false,
				Message: fmt.Sprintf("Failed to generate fixture: %v", err),
			}, nil
		}

		pbFixture := &pb.GeneratedFixture{
			Request: &pb.GeneratedRequest{
				Method:  fixture.Request.Method,
				Path:    fixture.Request.Path,
				Headers: fixture.Request.Headers,
			},
			Response: convertResponse(&fixture.Response),
		}

		if fixture.Request.Body != nil {
			reqJSON, err := json.MarshalIndent(fixture.Request.Body, "", "  ")
			if err == nil {
				pbFixture.Request.Body = string(reqJSON)
				if pbFixture.Request.Headers == nil {
					pbFixture.Request.Headers = make(map[string]string)
				}
				pbFixture.Request.Headers["Content-Type"] = opts.ContentType
			}
		}

		grpcRequestsTotal.WithLabelValues("GenerateFixture", "success").Inc()
		return &pb.GenerateFixtureResponse{
			Success: true,
			Message: "Fixture generated successfully",
			Fixture: pbFixture,
		}, nil
	}
}

// convertResponse converts a pkg/cvt GeneratedResponse to a protobuf GeneratedResponse.
func convertResponse(resp *cvt.GeneratedResponse) *pb.GeneratedResponse {
	pbResp := &pb.GeneratedResponse{
		StatusCode: int32(resp.StatusCode),
		Headers:    resp.Headers,
	}
	if resp.Body != nil {
		jsonData, err := json.MarshalIndent(resp.Body, "", "  ")
		if err == nil {
			pbResp.Body = string(jsonData)
		}
	}
	return pbResp
}
