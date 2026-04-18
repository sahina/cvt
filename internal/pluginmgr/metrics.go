package pluginmgr

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds the four Prometheus series the plugin manager exports.
// Use NewMetrics(nil) to register against the default registry, or pass
// a custom registerer for tests.
type Metrics struct {
	CallDuration *prometheus.HistogramVec
	CallErrors   *prometheus.CounterVec
	Up           *prometheus.GaugeVec
	Restarts     *prometheus.CounterVec
}

// ensure metrics register only once against the default registry across
// multiple manager instantiations in the same process (common in tests).
var (
	defaultMetricsOnce sync.Once
	defaultMetrics     *Metrics
)

// NewMetrics builds + registers the four plugin metrics. Passing nil uses
// prometheus.DefaultRegisterer and memoizes the result so a subsequent
// call returns the same Metrics struct (avoiding "duplicate metrics
// collector" panics across test binaries).
func NewMetrics(reg prometheus.Registerer) *Metrics {
	if reg == nil {
		defaultMetricsOnce.Do(func() {
			defaultMetrics = buildMetrics(prometheus.DefaultRegisterer)
		})
		return defaultMetrics
	}
	return buildMetrics(reg)
}

func buildMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		CallDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "cvt_plugin_call_duration_seconds",
			Help:    "Duration of plugin RPC calls in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"plugin", "service", "method"}),
		CallErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "cvt_plugin_call_errors_total",
			Help: "Total plugin RPC errors by canonical gRPC status code.",
		}, []string{"plugin", "service", "method", "code"}),
		Up: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "cvt_plugin_up",
			Help: "1 if the plugin subprocess is running and healthy, 0 otherwise.",
		}, []string{"plugin", "version"}),
		Restarts: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "cvt_plugin_restarts_total",
			Help: "Total number of times the plugin supervisor restarted a plugin subprocess.",
		}, []string{"plugin"}),
	}
	reg.MustRegister(m.CallDuration, m.CallErrors, m.Up, m.Restarts)
	return m
}
