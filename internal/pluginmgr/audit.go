package pluginmgr

import (
	"time"

	"go.uber.org/zap"
)

// AuditKind labels plugin-call audit entries by I/O direction.
type AuditKind string

const (
	AuditKindRead  AuditKind = "read"
	AuditKindWrite AuditKind = "write"
)

// AuditRecord is a single plugin-call audit entry. Fields mirror the
// design doc's audit schema: plugin identity (name + reported version +
// install-time sha256 + pid), correlation (request_id), the gRPC call
// (service/method), and outcome metrics (duration, status, error code).
type AuditRecord struct {
	Kind            AuditKind     `json:"kind"`
	Plugin          string        `json:"plugin"`
	ReportedVersion string        `json:"reported_version"`
	SHA256          string        `json:"sha256"`
	PID             int           `json:"pid"`
	RequestID       string        `json:"request_id"`
	Service         string        `json:"service"`
	Method          string        `json:"method"`
	DurationMS      int64         `json:"duration_ms"`
	Outcome         string        `json:"outcome"`
	ErrorCode       string        `json:"error_code,omitempty"`
	Timestamp       time.Time     `json:"timestamp"`
}

// Outcome values for AuditRecord.Outcome.
const (
	OutcomeOK      = "ok"
	OutcomeError   = "error"
	OutcomeTimeout = "timeout"
)

// AuditSink is the interface core plumbs audit records through. Server
// mode wires the existing server/cvtservice/audit_logger.go; CLI mode
// wires a Zap-backed sink. Tests wire an in-memory slice.
type AuditSink interface {
	Record(AuditRecord)
}

// ZapAuditSink adapts a Zap logger into an AuditSink. Writes are
// synchronous: the caller blocks until Zap enqueues the entry (Zap's own
// buffering is transparent). This preserves the compliance invariant
// that no plugin call returns to the caller until the audit entry is
// durably handed off.
type ZapAuditSink struct{ L *zap.Logger }

// Record emits the audit entry as a structured log at info level with a
// dedicated message so downstream filters can pick it out.
func (s ZapAuditSink) Record(r AuditRecord) {
	if s.L == nil {
		return
	}
	s.L.Info("plugin_audit",
		zap.String("kind", string(r.Kind)),
		zap.String("plugin", r.Plugin),
		zap.String("reported_version", r.ReportedVersion),
		zap.String("sha256", short(r.SHA256)),
		zap.Int("pid", r.PID),
		zap.String("request_id", r.RequestID),
		zap.String("service", r.Service),
		zap.String("method", r.Method),
		zap.Int64("duration_ms", r.DurationMS),
		zap.String("outcome", r.Outcome),
		zap.String("error_code", r.ErrorCode),
		zap.Time("ts", r.Timestamp),
	)
}

// NullAuditSink drops records. Use when plugins are disabled.
type NullAuditSink struct{}

func (NullAuditSink) Record(AuditRecord) {}

// RecordingAuditSink collects records in memory. Useful for tests.
type RecordingAuditSink struct {
	Records []AuditRecord
}

// Record appends the entry.
func (s *RecordingAuditSink) Record(r AuditRecord) {
	s.Records = append(s.Records, r)
}

// short returns the first 12 hex chars of a sha256 string, for compact
// audit fields. Short identifiers are preferred over full digests in logs
// to keep the output scannable.
func short(s string) string {
	if len(s) <= 12 {
		return s
	}
	return s[:12]
}
