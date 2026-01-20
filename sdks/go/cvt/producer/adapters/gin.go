package adapters

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/sahina/cvt/sdks/go/cvt/producer"
)

// GinMiddleware creates Gin middleware for producer validation.
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
//	// Create Gin router with middleware
//	router := gin.Default()
//	router.Use(adapters.GinMiddleware(config))
//
//	router.GET("/users", handleGetUsers)
//	router.POST("/users", handleCreateUser)
func GinMiddleware(config producer.Config) gin.HandlerFunc {
	p, err := producer.NewProducer(config)
	if err != nil {
		panic("cvt producer: " + err.Error())
	}

	return func(c *gin.Context) {
		// Check path filters
		if !config.ShouldValidatePath(c.Request.URL.Path) {
			c.Next()
			return
		}

		ctx := c.Request.Context()

		// Capture request body
		reqBody, err := captureGinRequestBody(c)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to read request body",
			})
			return
		}

		// Validate request (if enabled and not async)
		if config.ValidateRequest && !config.ShouldValidateAsync() {
			start := time.Now()
			result := p.ValidateRequest(ctx, c.Request, reqBody)
			recordValidationDuration("request", time.Since(start))

			if !result.Valid {
				if !handleGinRequestFailure(c, p, result) {
					recordRejection()
					return
				}
			}
		}

		// Async request validation for Shadow mode
		if config.ValidateRequest && config.ShouldValidateAsync() {
			req := c.Request.Clone(context.Background())
			go func() {
				result := p.ValidateRequest(context.Background(), req, reqBody)
				if !result.Valid {
					p.HandleRequestValidationFailure(nil, req, result)
				}
			}()
		}

		// Create response capture writer
		capture := &ginResponseCapture{
			ResponseWriter: c.Writer,
			body:           &bytes.Buffer{},
		}
		c.Writer = capture

		// Execute handler
		c.Next()

		// Validate response
		if config.ValidateResponse {
			if config.ShouldValidateAsync() {
				// Async response validation
				respBody := capture.body.Bytes()
				statusCode := capture.Status()
				respHeaders := capture.Header()
				req := c.Request.Clone(context.Background())
				go func() {
					result := p.ValidateResponse(context.Background(), req, reqBody, statusCode, respHeaders, respBody)
					if !result.Valid {
						p.HandleResponseValidationFailure(req, result)
					}
				}()
			} else {
				start := time.Now()
				result := p.ValidateResponse(ctx, c.Request, reqBody, capture.Status(), capture.Header(), capture.body.Bytes())
				recordValidationDuration("response", time.Since(start))

				if !result.Valid {
					p.HandleResponseValidationFailure(c.Request, result)
				}
			}
		}
	}
}

// captureGinRequestBody reads and restores the request body for Gin.
func captureGinRequestBody(c *gin.Context) ([]byte, error) {
	if c.Request.Body == nil {
		return nil, nil
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, err
	}

	// Restore the body for the handler
	c.Request.Body = io.NopCloser(bytes.NewReader(body))

	return body, nil
}

// handleGinRequestFailure handles request validation failure for Gin.
func handleGinRequestFailure(c *gin.Context, p *producer.Producer, result *producer.ValidationResult) bool {
	// Call the producer's handler (which may use custom callbacks)
	continueProcessing := p.HandleRequestValidationFailure(nil, c.Request, result)

	if !continueProcessing {
		// Abort with the error response
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error":   "Request validation failed",
			"details": result.Errors,
		})
		return false
	}

	return true
}

// ginResponseCapture wraps gin.ResponseWriter to capture the response body.
type ginResponseCapture struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

// Write captures the body while writing to the underlying writer.
func (w *ginResponseCapture) Write(data []byte) (int, error) {
	w.body.Write(data)
	return w.ResponseWriter.Write(data)
}

// WriteString captures the body while writing to the underlying writer.
func (w *ginResponseCapture) WriteString(s string) (int, error) {
	w.body.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}
