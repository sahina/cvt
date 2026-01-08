package producer

import "errors"

var (
	// ErrSchemaIDRequired is returned when no schema ID is configured.
	ErrSchemaIDRequired = errors.New("producer: schema ID is required")

	// ErrValidatorRequired is returned when no validator is configured.
	ErrValidatorRequired = errors.New("producer: validator is required (embedded or gRPC)")

	// ErrValidationFailed is returned when validation fails.
	ErrValidationFailed = errors.New("producer: validation failed")
)
