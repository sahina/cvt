package adapters

import (
	"net/http"

	"github.com/cvt/cvt-sdk/go/cvt/producer"
)

// ChiMiddleware creates Chi middleware for producer validation.
//
// Chi uses the standard http.Handler interface, so this is a thin wrapper
// around NetHTTPMiddleware that returns the correct type signature for Chi.
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
//	// Create Chi router with middleware
//	r := chi.NewRouter()
//	r.Use(adapters.ChiMiddleware(config))
//
//	r.Get("/users", handleGetUsers)
//	r.Post("/users", handleCreateUser)
func ChiMiddleware(config producer.Config) func(http.Handler) http.Handler {
	// Chi uses the same signature as net/http, so we can reuse NetHTTPMiddleware
	return NetHTTPMiddleware(config)
}
