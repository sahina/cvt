// Package main provides structured logging functionality for the Contract Validation Tool.
// This file wraps the zap logging library with convenience functions and configures
// logging for both development and production environments.
package cvtservice

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// logger is the global logger instance used throughout the application.
// It should be initialized once at startup using InitLogger.
var logger *zap.Logger

// InitLogger initializes the global logger with production or development configuration.
// The configuration varies based on the environment:
//
// Development mode (development=true):
// - Console output with colorized log levels
// - Pretty-printed, human-readable format
// - Includes caller information
// - More verbose output
//
// Production mode (development=false):
// - JSON output for structured logging
// - ISO8601 timestamps
// - Optimized for machine parsing
// - Includes caller information
//
// Parameters:
//   - development: If true, uses development config; otherwise uses production config
//
// Returns:
//   - error: An error if logger initialization fails
func InitLogger(development bool) error {
	var err error
	var config zap.Config

	if development {
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		config = zap.NewProductionConfig()
		config.EncoderConfig.TimeKey = "timestamp"
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	}

	logger, err = config.Build(
		zap.AddCallerSkip(1), // Skip one level to show the actual caller
	)
	if err != nil {
		return err
	}

	return nil
}

// GetLogger returns the global logger instance.
// If the logger has not been initialized, it creates a fallback production logger.
//
// Returns:
//   - *zap.Logger: The global logger instance
func GetLogger() *zap.Logger {
	if logger == nil {
		// Fallback to a no-op logger if not initialized
		logger, _ = zap.NewProduction()
	}
	return logger
}

// Info logs an informational message.
// Use this for general operational messages about what the application is doing.
//
// Parameters:
//   - msg: The log message
//   - fields: Optional structured fields (e.g., zap.String("key", "value"))
func Info(msg string, fields ...zap.Field) {
	GetLogger().Info(msg, fields...)
}

// Debug logs a debug message.
// Use this for detailed diagnostic information useful during development.
// Debug messages are typically disabled in production.
//
// Parameters:
//   - msg: The log message
//   - fields: Optional structured fields (e.g., zap.String("key", "value"))
func Debug(msg string, fields ...zap.Field) {
	GetLogger().Debug(msg, fields...)
}

// Warn logs a warning message.
// Use this for potentially harmful situations that don't prevent operation.
//
// Parameters:
//   - msg: The log message
//   - fields: Optional structured fields (e.g., zap.String("key", "value"))
func Warn(msg string, fields ...zap.Field) {
	GetLogger().Warn(msg, fields...)
}

// Error logs an error message.
// Use this for error conditions that should be investigated but don't require shutdown.
//
// Parameters:
//   - msg: The log message
//   - fields: Optional structured fields (e.g., zap.Error(err))
func Error(msg string, fields ...zap.Field) {
	GetLogger().Error(msg, fields...)
}

// Fatal logs a fatal message and exits the application.
// Use this for critical errors that make the application unable to continue.
// This function calls os.Exit(1) after logging.
//
// Parameters:
//   - msg: The log message
//   - fields: Optional structured fields (e.g., zap.Error(err))
func Fatal(msg string, fields ...zap.Field) {
	GetLogger().Fatal(msg, fields...)
}

// Sync flushes any buffered log entries.
// This should be called before the application exits to ensure all logs are written.
// It's safe to call even if the logger hasn't been initialized.
//
// Returns:
//   - error: An error if flushing fails
func Sync() error {
	if logger != nil {
		return logger.Sync()
	}
	return nil
}
