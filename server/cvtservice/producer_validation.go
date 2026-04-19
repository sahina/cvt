package cvtservice

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/sahina/cvt/server/pb"
	"github.com/sahina/cvt/server/storage"
	"go.uber.org/zap"
)

// ValidateProducerResponse validates a producer's response against the registered schema.
// Producers (API implementations) call this to verify that the responses they generate
// conform to their published OpenAPI contract before shipping.
func (s *ValidatorService) ValidateProducerResponse(ctx context.Context, req *pb.ValidateProducerRequest) (*pb.ValidationResult, error) {
	// Handle nil request early to avoid panic
	if req == nil {
		return &pb.ValidationResult{
			Valid:  false,
			Errors: []string{"ValidateProducerRequest cannot be null"},
		}, nil
	}

	// Record metrics for validation timing
	start := time.Now()
	schemaID := req.GetSchemaId()
	method := req.GetMethod()

	defer func() {
		validationDuration.WithLabelValues(schemaID, method).Observe(time.Since(start).Seconds())
		grpcRequestDuration.WithLabelValues("ValidateProducerResponse").Observe(time.Since(start).Seconds())
	}()

	Debug("Received ValidateProducerResponse request",
		zap.String("schemaId", schemaID),
		zap.String("method", method),
		zap.String("path", req.Path))

	// Validate inputs
	if err := s.validateProducerRequest(req); err != nil {
		Warn("Validation error in ValidateProducerResponse",
			zap.String("schemaId", schemaID),
			zap.Error(err))
		validationsTotal.WithLabelValues(schemaID, method, "error").Inc()
		validationErrors.WithLabelValues("input_validation").Inc()
		grpcRequestsTotal.WithLabelValues("ValidateProducerResponse", "failure").Inc()
		return &pb.ValidationResult{
			Valid:  false,
			Errors: []string{fmt.Sprintf("Validation error: %v", err)},
		}, nil
	}

	// Retrieve schema (cache first, then storage)
	entry, found := s.getSchemaEntry(ctx, schemaID, req.SchemaVersion)
	if !found || entry == nil {
		Warn("Schema not found", zap.String("schemaId", schemaID))
		validationsTotal.WithLabelValues(schemaID, method, "error").Inc()
		validationErrors.WithLabelValues("schema_not_found").Inc()
		grpcRequestsTotal.WithLabelValues("ValidateProducerResponse", "failure").Inc()
		return &pb.ValidationResult{
			Valid:  false,
			Errors: []string{fmt.Sprintf("Schema not found: %s", schemaID)},
		}, nil
	}
	doc := entry.Document

	// Handle basePath from Swagger 2.0 schemas and resolve base URL
	requestPath := stripBasePath(doc, req.Path)
	baseURL := resolveBaseURL(doc)

	// Create http.Request for route matching (even though we're only validating response)
	httpReq, err := http.NewRequest(strings.ToUpper(method), fmt.Sprintf("%s%s", baseURL, requestPath), nil)
	if err != nil {
		Error("Failed to create HTTP request for route matching", zap.Error(err))
		validationsTotal.WithLabelValues(schemaID, method, "error").Inc()
		validationErrors.WithLabelValues("http_request_creation").Inc()
		grpcRequestsTotal.WithLabelValues("ValidateProducerResponse", "failure").Inc()
		return &pb.ValidationResult{
			Valid:  false,
			Errors: []string{fmt.Sprintf("Failed to create HTTP request: %v", err)},
		}, nil
	}

	// Add request headers if provided (for context)
	if req.Request != nil && req.Request.Headers != nil {
		for key, value := range req.Request.Headers {
			httpReq.Header.Set(key, value)
		}
	}

	// Use the pre-built router from the schema entry (built at registration time).
	// Use a local variable to avoid mutating the shared cache entry (data race).
	router := entry.Router
	if router == nil {
		var routerErr error
		router, routerErr = s.buildRouter(doc)
		if routerErr != nil {
			Error("Failed to create router",
				zap.String("schemaId", schemaID),
				zap.Error(routerErr))
			validationsTotal.WithLabelValues(schemaID, method, "error").Inc()
			validationErrors.WithLabelValues("router_creation").Inc()
			grpcRequestsTotal.WithLabelValues("ValidateProducerResponse", "failure").Inc()
			return &pb.ValidationResult{
				Valid:  false,
				Errors: []string{fmt.Sprintf("Failed to create router: %v", routerErr)},
			}, nil
		}
	}

	// Find the matching route
	route, pathParams, err := router.FindRoute(httpReq)
	if err != nil {
		Error("Route not found",
			zap.String("method", method),
			zap.String("path", req.Path),
			zap.Error(err))
		validationsTotal.WithLabelValues(schemaID, method, "invalid").Inc()
		validationErrors.WithLabelValues("route_not_found").Inc()
		grpcRequestsTotal.WithLabelValues("ValidateProducerResponse", "success").Inc()
		return &pb.ValidationResult{
			Valid:  false,
			Errors: []string{fmt.Sprintf("Route not found: %s %s - %v", method, req.Path, err)},
		}, nil
	}

	// Create request validation input (needed for response validation context)
	requestValidationInput := &openapi3filter.RequestValidationInput{
		Request:    httpReq,
		PathParams: pathParams,
		Route:      route,
	}

	// Validate the response against the OpenAPI schema
	responseValidationInput := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: requestValidationInput,
		Status:                 int(req.Response.StatusCode),
		Header:                 s.createHTTPHeaders(req.Response.Headers),
		Body:                   io.NopCloser(strings.NewReader(req.Response.Body)),
	}

	if err := openapi3filter.ValidateResponse(ctx, responseValidationInput); err != nil {
		Debug("Response validation failed",
			zap.Int32("statusCode", req.Response.StatusCode),
			zap.Error(err))
		validationsTotal.WithLabelValues(schemaID, method, "invalid").Inc()
		validationErrors.WithLabelValues("response_invalid").Inc()
		grpcRequestsTotal.WithLabelValues("ValidateProducerResponse", "success").Inc()
		return &pb.ValidationResult{
			Valid:  false,
			Errors: []string{err.Error()},
		}, nil
	}

	Info("Producer response validated successfully",
		zap.String("schemaId", schemaID),
		zap.String("method", method),
		zap.String("path", req.Path),
		zap.Int32("statusCode", req.Response.StatusCode))

	// Record successful validation
	validationsTotal.WithLabelValues(schemaID, method, "valid").Inc()
	grpcRequestsTotal.WithLabelValues("ValidateProducerResponse", "success").Inc()

	// Include version and hash in result
	producerResult := &pb.ValidationResult{
		Valid:  true,
		Errors: nil,
	}
	if entry.Metadata != nil {
		producerResult.ValidatedAgainstVersion = entry.Metadata.SchemaVersion
		producerResult.ValidatedAgainstHash = entry.Metadata.SchemaHash
	}

	// Asynchronously record validation to storage
	if s.store != nil {
		go func() {
			record := &storage.ValidationRecord{
				SchemaID:       schemaID,
				SchemaVersion:  entry.Metadata.SchemaVersion,
				SchemaHash:     entry.Metadata.SchemaHash,
				RequestMethod:  method,
				RequestPath:    req.Path,
				ResponseStatus: req.Response.StatusCode,
				Valid:          producerResult.Valid,
				Errors:         producerResult.Errors,
				DurationMs:     time.Since(start).Milliseconds(),
				ValidatedAt:    time.Now(),
			}
			if recErr := s.store.RecordValidation(context.Background(), record); recErr != nil {
				Warn("Failed to record validation", zap.Error(recErr))
			}
		}()
	}

	return producerResult, nil
}

// validateProducerRequest validates the ValidateProducerRequest.
func (s *ValidatorService) validateProducerRequest(req *pb.ValidateProducerRequest) error {
	if req == nil {
		return fmt.Errorf("ValidateProducerRequest cannot be null")
	}
	if err := ValidateSchemaID(req.SchemaId); err != nil {
		return err
	}
	if err := ValidateHTTPMethod(req.Method); err != nil {
		return err
	}
	if err := ValidateHTTPPath(req.Path); err != nil {
		return err
	}
	if req.Response == nil {
		return fmt.Errorf("ResponseData cannot be null")
	}
	if err := ValidateStatusCode(req.Response.StatusCode); err != nil {
		return err
	}
	if err := ValidateResponseBody(req.Response.Body); err != nil {
		return err
	}
	return nil
}
