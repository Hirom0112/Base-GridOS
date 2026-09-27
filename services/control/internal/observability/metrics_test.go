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
	if err := metrics.RecordCommand("SENT"); err != nil {
		t.Fatal(err)
	}
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

func TestMetricsRejectPrivateLabelsAndInvalidPopulation(t *testing.T) {
	t.Parallel()
	metrics := NewMetrics()
	if err := metrics.RecordCommand("site-private-123"); err == nil {
		t.Fatal("private command label accepted")
	}
	if err := metrics.SetTelemetryPopulation(4, 5); err == nil {
		t.Fatal("stale count above total accepted")
	}
	if err := metrics.RecordCommand("SENT"); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/metrics", nil))
	if strings.Contains(response.Body.String(), "site-private-123") {
		t.Fatalf("private label exported: %s", response.Body.String())
	}
}
