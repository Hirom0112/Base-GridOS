package observability

import (
	"errors"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	registry           *prometheus.Registry
	commands           *prometheus.CounterVec
	ackLatency         prometheus.Histogram
	telemetryFreshness prometheus.Gauge
	telemetryDevices   prometheus.Gauge
	staleDevices       prometheus.Gauge
	safetyRejections   prometheus.Counter
	solverDuration     prometheus.Histogram
	fallbacks          prometheus.Counter
	uncertainCommands  prometheus.Gauge
}

var ProcessMetrics = NewMetrics()

func NewMetrics() *Metrics {
	m := &Metrics{
		registry: prometheus.NewRegistry(),
		commands: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gridos_commands_total", Help: "Commands entering each state.",
		}, []string{"state"}),
		ackLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name: "gridos_ack_latency_seconds", Help: "Time from command send to acknowledgement.",
			Buckets: prometheus.ExponentialBuckets(0.01, 2, 12),
		}),
		telemetryFreshness: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gridos_telemetry_freshness_seconds", Help: "Age of the latest accepted telemetry observation.",
		}),
		telemetryDevices: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gridos_telemetry_devices", Help: "Devices in the telemetry population.",
		}),
		staleDevices: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gridos_telemetry_stale_devices", Help: "Devices with stale telemetry.",
		}),
		safetyRejections: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gridos_safety_rejections_total", Help: "Plans rejected by the independent safety gate.",
		}),
		solverDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name: "gridos_solver_duration_seconds", Help: "Time spent in the optimization solver.",
			Buckets: prometheus.ExponentialBuckets(0.01, 2, 14),
		}),
		fallbacks: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gridos_fallback_total", Help: "Dispatches that used a fallback plan.",
		}),
		uncertainCommands: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gridos_uncertain_commands", Help: "Commands with uncertain acknowledgement.",
		}),
	}
	m.registry.MustRegister(m.commands, m.ackLatency, m.telemetryFreshness, m.telemetryDevices,
		m.staleDevices, m.safetyRejections, m.solverDuration, m.fallbacks, m.uncertainCommands)
	return m
}

func NewEventMetrics() *Metrics {
	m := NewMetrics()
	m.registry.Unregister(m.telemetryFreshness)
	m.registry.Unregister(m.telemetryDevices)
	m.registry.Unregister(m.staleDevices)
	m.registry.Unregister(m.uncertainCommands)
	return m
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

func (m *Metrics) RecordCommand(state string) error {
	switch state {
	case "PERSISTED", "PENDING", "SENT", "ACKNOWLEDGED", "UNCERTAIN", "EXECUTING", "COMPLETED", "DELIVERED", "EXPIRED", "REJECTED", "CANCELLED":
		m.commands.WithLabelValues(state).Inc()
		return nil
	default:
		return errors.New("unknown command state")
	}
}

func (m *Metrics) ObserveAckLatency(duration time.Duration) error {
	if duration < 0 {
		return errors.New("negative acknowledgement latency")
	}
	m.ackLatency.Observe(duration.Seconds())
	return nil
}

func (m *Metrics) SetTelemetryFreshness(age time.Duration) error {
	if age < 0 {
		return errors.New("negative telemetry age")
	}
	m.telemetryFreshness.Set(age.Seconds())
	return nil
}

func (m *Metrics) SetTelemetryPopulation(total, stale int) error {
	if total < 0 || stale < 0 || stale > total {
		return errors.New("invalid telemetry population")
	}
	m.telemetryDevices.Set(float64(total))
	m.staleDevices.Set(float64(stale))
	return nil
}

func (m *Metrics) RecordSafetyRejection() {
	m.safetyRejections.Inc()
}

func (m *Metrics) ObserveSolverTime(duration time.Duration) error {
	if duration < 0 {
		return errors.New("negative solver duration")
	}
	m.solverDuration.Observe(duration.Seconds())
	return nil
}

func (m *Metrics) RecordFallback() {
	m.fallbacks.Inc()
}

func (m *Metrics) SetUncertainCommands(count int) error {
	if count < 0 {
		return errors.New("negative uncertain command count")
	}
	m.uncertainCommands.Set(float64(count))
	return nil
}
