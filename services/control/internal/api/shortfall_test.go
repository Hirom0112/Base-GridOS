package api

import (
	"context"
	"encoding/json"
	"testing"
)

func TestStoredDeliveryShortfallUsesVerificationCoverage(t *testing.T) {
	pool := apiTestDatabase(t)
	_, err := pool.Exec(context.Background(), `INSERT INTO dispatch_requests
		(request_id, event_type, begin_time, end_time, target_kw, measurement_boundary, load_zones, correlation_id)
		VALUES ('request-shortfall', 'GRID_SERVICE', now(), now() + interval '10 minutes', 1000, 'METER_NET_EXPORT', ARRAY['LZ_AEN'], 'shortfall');
		INSERT INTO dispatch_events (event_id, request_id, state, plan_version, correlation_id)
		VALUES ('event-shortfall', 'request-shortfall', 'REQUESTED', 0, 'shortfall');
		INSERT INTO verification_summaries (verification_id, event_id, interval_begin_time, interval_end_time,
		requested_kw, commanded_kw, delivered_kw, tracking_error_kw, confidence, baseline_method, measurement_boundary, correlation_id)
		VALUES ('verify-shortfall-1', 'event-shortfall', now(), now() + interval '5 minutes', 1000, 0, 0, 0, 1, 'MEASURED_AT_BOUNDARY', 'METER_NET_EXPORT', 'shortfall'),
		('verify-shortfall-2', 'event-shortfall', now() + interval '5 minutes', now() + interval '10 minutes', 1000, 0, 0, 0, 0, 'MEASURED_AT_BOUNDARY', 'METER_NET_EXPORT', 'shortfall')`)
	if err != nil {
		t.Fatal(err)
	}
	report, err := NewPostgresReportSource(pool).EventReportData(context.Background(), "event-shortfall")
	if err != nil {
		t.Fatal(err)
	}
	if report.RequestedMW != 1 {
		t.Fatal("stored request control missing")
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &body); err != nil {
		t.Fatal(err)
	}
	var intervals []map[string]json.RawMessage
	if err := json.Unmarshal(body["delivery_shortfall"], &intervals); err != nil || len(intervals) != 2 || string(intervals[0]["measured_delivered_kwh"]) != "0" || string(intervals[0]["coverage"]) != "1" || string(intervals[1]["coverage"]) != "0" || intervals[1]["shortfall_kwh"] != nil {
		t.Fatalf("stored delivery shortfall = %s", encoded)
	}
}
