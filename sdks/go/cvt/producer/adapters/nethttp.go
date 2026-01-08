// Package adapters provides framework-specific middleware for producer validation.
package adapters

import (
	"context"
	"net/http"
	"time"

	"github.com/cvt/cvt-sdk/go/cvt/producer"
)

// NetHTTPMiddleware creates HTTP middleware for the standard library.
//
// This middleware validates incoming requests before they reach the handler
// and validates outgoing responses after the handler completes.
//
// Example:
//
//	// Create embedded validator
//	validator := embeddedcvt.NewValidator()
//	validator.RegisterSchemaFromFile("my-api", "./openapi.json")
//
//	// Create producer config
//	config := producer.Config{
//	    SchemaID:          "my-api",
//	    EmbeddedValidator: validator,
//	    Mode:              producer.ModeStrict,
//	}
//
//	// Create middleware
//	middleware := adapters.NetHTTPMiddleware(config)
//
//	// Wrap your handler
//	http.Handle("/", middleware(myHandler))
func NetHTTPMiddleware(config producer.Config) func(http.Handler) http.Handler {
	p, err := producer.NewProducer(config)
	if err != nil {
		panic("cvt producer: " + err.Error())
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check path filters
			if !config.ShouldValidatePath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			ctx := r.Context()

			// Capture request body
			reqBody, err := producer.CaptureRequestBody(r)
			if err != nil {
				http.Error(w, "Failed to read request body", http.StatusInternalServerError)
				return
			}

			// Validate request (if enabled and not async)
			if config.ValidateRequest && !config.ShouldValidateAsync() {
				start := time.Now()
				result := p.ValidateRequest(ctx, r, reqBody)
				recordValidationDuration("request", time.Since(start))

				if !result.Valid {
					if !p.HandleRequestValidationFailure(w, r, result) {
						recordRejection()
						return
					}
				}
			}

			// Async request validation for Shadow mode
			if config.ValidateRequest && config.ShouldValidateAsync() {
				go func() {
					asyncCtx := context.Background()
					result := p.ValidateRequest(asyncCtx, r, reqBody)
					if !result.Valid {
						p.HandleRequestValidationFailure(nil, r, result)
					}
				}()
			}

			// Wrap response writer to capture response
			capture := producer.NewResponseCapture(w)

			// Execute handler
			next.ServeHTTP(capture, r)

			// Validate response
			if config.ValidateResponse {
				if config.ShouldValidateAsync() {
					// Async response validation
					respBody := capture.Body.Bytes()
					statusCode := capture.StatusCode
					respHeaders := capture.Header()
					go func() {
						asyncCtx := context.Background()
						result := p.ValidateResponse(asyncCtx, r, reqBody, statusCode, respHeaders, respBody)
						if !result.Valid {
							p.HandleResponseValidationFailure(r, result)
						}
					}()
				} else {
					start := time.Now()
					result := p.ValidateResponse(ctx, r, reqBody, capture.StatusCode, capture.Header(), capture.Body.Bytes())
					recordValidationDuration("response", time.Since(start))

					if !result.Valid {
						p.HandleResponseValidationFailure(r, result)
					}
				}
			}
		})
	}
}

// recordValidationDuration records validation duration in metrics.
func recordValidationDuration(validationType string, duration time.Duration) {
	producer.RecordDuration(validationType, duration)
}

// recordRejection records a rejected request.
func recordRejection() {
	producer.RecordRejection()
}
