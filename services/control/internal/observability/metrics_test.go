package observability

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMetricsExposeDashboardNames(t *testing.T) {
	t.Parallel()
	metrics := NewMetrics()
	metrics.RecordCommand(CommandSent)
	if err := metrics.ObserveAckLatency(120 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if err := metrics.SetTelemetryFreshness(3 * time.Second); err != nil {
		t.Fatal(err)
	}
	if err := metrics.SetTelemetryPopulation(100, 4); err != nil {
		t.Fatal(err)
	}
	metrics.RecordSafetyRejection()
	if err := metrics.ObserveSolverTime(240 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	metrics.RecordFallback()
	if err := metrics.SetUncertainCommands(2); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/metrics", nil))
	if response.Code != 200 {
		t.Fatalf("metrics status: %d", response.Code)
	}
	for _, name := range []string{
		"gridos_commands_total", "gridos_ack_latency_seconds_bucket",
		"gridos_telemetry_freshness_seconds", "gridos_telemetry_devices",
		"gridos_telemetry_stale_devices", "gridos_safety_rejections_total",
		"gridos_solver_duration_seconds_bucket", "gridos_fallback_total",
		"gridos_uncertain_commands",
	} {
		if !strings.Contains(response.Body.String(), name) {
			t.Fatalf("missing %s from metrics", name)
		}
	}
	if !strings.Contains(response.Body.String(), `gridos_commands_total{state="SENT"} 1`) {
		t.Fatalf("command state missing: %s", response.Body.String())
	}
}

func TestMetricsRejectInvalidPopulation(t *testing.T) {
	t.Parallel()
	metrics := NewMetrics()
	if err := metrics.SetTelemetryPopulation(4, 5); err == nil {
		t.Fatal("stale count above total accepted")
	}
	if err := metrics.SetTelemetryPopulation(5, 4); err != nil {
		t.Fatal(err)
	}
}

func TestRecordCommandAcceptsEveryCommandState(t *testing.T) {
	t.Parallel()
	metrics := NewMetrics()
	for state := CommandPersisted; state < commandStateCount; state++ {
		metrics.RecordCommand(state)
	}
	response := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/metrics", nil))
	for _, label := range []string{"PERSISTED", "SENT", "ACKNOWLEDGED", "UNCERTAIN", "EXECUTING", "COMPLETED", "EXPIRED", "REJECTED", "CANCELLED"} {
		if !strings.Contains(response.Body.String(), `gridos_commands_total{state="`+label+`"} 1`) {
			t.Fatalf("missing command state %s: %s", label, response.Body.String())
		}
	}
}

func TestEventMetricsOmitDatabaseStateGauges(t *testing.T) {
	metrics := NewEventMetrics()
	metrics.RecordCommand(CommandSent)
	response := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/metrics", nil))
	if !strings.Contains(response.Body.String(), "gridos_commands_total") {
		t.Fatal("event counter missing")
	}
	for _, name := range []string{"gridos_telemetry_freshness_seconds", "gridos_telemetry_devices", "gridos_telemetry_stale_devices", "gridos_uncertain_commands"} {
		if strings.Contains(response.Body.String(), name) {
			t.Fatalf("database gauge %s exposed by worker", name)
		}
	}
}
