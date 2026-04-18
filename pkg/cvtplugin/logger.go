package cvtplugin

import (
	"io"
	"log"

	"github.com/hashicorp/go-hclog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewHCLogFromZap returns an hclog.Logger that forwards to a Zap logger.
// CVT core uses this to wire plugin log output (which arrives as hclog
// calls via hashicorp/go-plugin's native forwarding) into the project's
// existing Zap-based logger.
//
// The returned logger's With/Named methods create sub-loggers that
// carry additional fields through to Zap.
func NewHCLogFromZap(z *zap.Logger, pluginName string) hclog.Logger {
	if z == nil {
		z = zap.NewNop()
	}
	return &zapHclog{
		z:    z.With(zap.String("plugin", pluginName)),
		name: pluginName,
	}
}

type zapHclog struct {
	z    *zap.Logger
	name string
}

func (h *zapHclog) Log(level hclog.Level, msg string, args ...interface{}) {
	fields := argsToFields(args)
	switch level {
	case hclog.Trace, hclog.Debug:
		h.z.Debug(msg, fields...)
	case hclog.Info:
		h.z.Info(msg, fields...)
	case hclog.Warn:
		h.z.Warn(msg, fields...)
	case hclog.Error:
		h.z.Error(msg, fields...)
	default:
		h.z.Info(msg, fields...)
	}
}

func (h *zapHclog) Trace(msg string, args ...interface{}) { h.Log(hclog.Trace, msg, args...) }
func (h *zapHclog) Debug(msg string, args ...interface{}) { h.Log(hclog.Debug, msg, args...) }
func (h *zapHclog) Info(msg string, args ...interface{})  { h.Log(hclog.Info, msg, args...) }
func (h *zapHclog) Warn(msg string, args ...interface{})  { h.Log(hclog.Warn, msg, args...) }
func (h *zapHclog) Error(msg string, args ...interface{}) { h.Log(hclog.Error, msg, args...) }

func (h *zapHclog) IsTrace() bool { return h.z.Core().Enabled(zapcore.DebugLevel) }
func (h *zapHclog) IsDebug() bool { return h.z.Core().Enabled(zapcore.DebugLevel) }
func (h *zapHclog) IsInfo() bool  { return h.z.Core().Enabled(zapcore.InfoLevel) }
func (h *zapHclog) IsWarn() bool  { return h.z.Core().Enabled(zapcore.WarnLevel) }
func (h *zapHclog) IsError() bool { return h.z.Core().Enabled(zapcore.ErrorLevel) }

func (h *zapHclog) ImpliedArgs() []interface{} { return nil }

func (h *zapHclog) With(args ...interface{}) hclog.Logger {
	return &zapHclog{z: h.z.With(argsToFields(args)...), name: h.name}
}

func (h *zapHclog) Name() string { return h.name }

func (h *zapHclog) Named(name string) hclog.Logger {
	full := name
	if h.name != "" {
		full = h.name + "." + name
	}
	return &zapHclog{z: h.z.Named(name), name: full}
}

func (h *zapHclog) ResetNamed(name string) hclog.Logger {
	return &zapHclog{z: h.z.Named(name), name: name}
}

func (h *zapHclog) SetLevel(_ hclog.Level) {
	// Zap levels are static per logger; honoring runtime level changes would
	// require an AtomicLevel wrapper. No-op is fine for CVT: the wrapping
	// Zap logger owns its level.
}

func (h *zapHclog) GetLevel() hclog.Level {
	switch {
	case h.z.Core().Enabled(zapcore.DebugLevel):
		return hclog.Debug
	case h.z.Core().Enabled(zapcore.InfoLevel):
		return hclog.Info
	case h.z.Core().Enabled(zapcore.WarnLevel):
		return hclog.Warn
	case h.z.Core().Enabled(zapcore.ErrorLevel):
		return hclog.Error
	}
	return hclog.NoLevel
}

func (h *zapHclog) StandardLogger(_ *hclog.StandardLoggerOptions) *log.Logger {
	return log.New(h.StandardWriter(nil), "", 0)
}

func (h *zapHclog) StandardWriter(_ *hclog.StandardLoggerOptions) io.Writer {
	return stdLogShim{z: h.z}
}

// argsToFields converts hclog's key/value variadic args into Zap fields.
// hclog uses flat alternating pairs: ["key1", value1, "key2", value2, ...].
func argsToFields(args []interface{}) []zap.Field {
	if len(args) == 0 {
		return nil
	}
	fields := make([]zap.Field, 0, len(args)/2)
	for i := 0; i < len(args); i += 2 {
		key, ok := args[i].(string)
		if !ok {
			key = "arg"
		}
		var val interface{}
		if i+1 < len(args) {
			val = args[i+1]
		}
		fields = append(fields, zap.Any(key, val))
	}
	return fields
}

type stdLogShim struct{ z *zap.Logger }

func (s stdLogShim) Write(p []byte) (int, error) {
	s.z.Info(string(p))
	return len(p), nil
}
