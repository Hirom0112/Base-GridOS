package api

import (
	"context"
	"encoding/json"
	"math"
	"testing"
)

func TestStoredDeliveryShortfallUsesVerificationCoverage(t *testing.T) {
	pool := apiTestDatabase(t)
	_, err := pool.Exec(context.Background(), `INSERT INTO dispatch_requests
		(request_id, event_type, begin_time, end_time, target_kw, measurement_boundary, load_zones, correlation_id)
		VALUES ('request-shortfall', 'GRID_SERVICE', now(), now() + interval '15 minutes', 1000, 'METER_NET_EXPORT', ARRAY['LZ_AEN'], 'shortfall');
		INSERT INTO dispatch_events (event_id, request_id, state, plan_version, correlation_id)
		VALUES ('event-shortfall', 'request-shortfall', 'REQUESTED', 0, 'shortfall');
		INSERT INTO verification_summaries (verification_id, event_id, interval_begin_time, interval_end_time,
		requested_kw, commanded_kw, delivered_kw, tracking_error_kw, confidence, baseline_method, measurement_boundary, correlation_id, measured_delivered_kwh)
		VALUES ('verify-shortfall-1', 'event-shortfall', now(), now() + interval '5 minutes', 1000, 0, 0, 0, 1, 'MEASURED_AT_BOUNDARY', 'METER_NET_EXPORT', 'shortfall', 0),
		('verify-shortfall-2', 'event-shortfall', now() + interval '5 minutes', now() + interval '10 minutes', 1000, 0, 0, 0, 0, 'MEASURED_AT_BOUNDARY', 'METER_NET_EXPORT', 'shortfall', NULL),
		('verify-shortfall-3', 'event-shortfall', now() + interval '10 minutes', now() + interval '15 minutes', 1000, 0, 1000, 0, 0.5, 'MEASURED_AT_BOUNDARY', 'METER_NET_EXPORT', 'shortfall', 25)`)
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
	if err := json.Unmarshal(body["delivery_shortfall"], &intervals); err != nil || len(intervals) != 3 || string(intervals[0]["measured_delivered_kwh"]) != "0" || string(intervals[0]["coverage"]) != "1" || string(intervals[1]["coverage"]) != "0" || intervals[1]["measured_delivered_kwh"] != nil || intervals[1]["shortfall_kwh"] != nil || string(intervals[2]["coverage"]) != "0.5" || string(intervals[2]["measured_delivered_kwh"]) != "25" {
		t.Fatalf("stored delivery shortfall = %s", encoded)
	}
	var requested, shortfall float64
	if err := json.Unmarshal(intervals[2]["requested_kwh"], &requested); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(intervals[2]["shortfall_kwh"], &shortfall); err != nil || math.Abs(shortfall-(requested-25)) > 1e-9 {
		t.Fatalf("partial measured shortfall = %s", encoded)
	}
}

func TestStoredPlannedShortfallUsesApprovedPlanVersion(t *testing.T) {
	pool := apiTestDatabase(t)
	_, err := pool.Exec(context.Background(), `INSERT INTO dispatch_requests
		(request_id, event_type, begin_time, end_time, target_kw, measurement_boundary, load_zones, correlation_id)
		VALUES ('request-plan-shortfall', 'GRID_SERVICE', now(), now() + interval '5 minutes', 1000, 'METER_NET_EXPORT', ARRAY['LZ_AEN'], 'plan-shortfall');
		INSERT INTO dispatch_events (event_id, request_id, state, plan_version, correlation_id)
		VALUES ('event-plan-shortfall', 'request-plan-shortfall', 'VALIDATED', 2, 'plan-shortfall');
		INSERT INTO input_snapshots (snapshot_id, event_id, captured_at, inputs, provenance, correlation_id)
		VALUES ('input-plan-shortfall', 'event-plan-shortfall', now(), '{}', '{}', 'plan-shortfall');
		INSERT INTO eligibility_snapshots (snapshot_id, event_id, captured_at, eligible_device_ids, exclusions, policy_version, correlation_id)
		VALUES ('eligibility-plan-shortfall', 'event-plan-shortfall', now(), ARRAY[]::text[], '[]', 'policy-plan-shortfall', 'plan-shortfall');
		INSERT INTO plan_versions (event_id, version, input_snapshot_id, eligibility_snapshot_id, plan, solver_version, model_version, correlation_id)
		VALUES ('event-plan-shortfall', 1, 'input-plan-shortfall', 'eligibility-plan-shortfall',
		'{"eventId":"event-plan-shortfall","planVersion":"1","shortfalls":[{"intervalBeginTime":"2026-09-27T10:00:00Z","intervalEndTime":"2026-09-27T10:05:00Z","requestedKw":1000,"feasibleKw":800,"shortfallKw":200,"reasons":["RESERVE"]}]}', 'solver-approved', 'model-approved', 'plan-shortfall'),
		('event-plan-shortfall', 2, 'input-plan-shortfall', 'eligibility-plan-shortfall',
		'{"eventId":"event-plan-shortfall","planVersion":"2","shortfalls":[{"intervalBeginTime":"2026-09-27T10:00:00Z","intervalEndTime":"2026-09-27T10:05:00Z","requestedKw":1000,"feasibleKw":500,"shortfallKw":500,"reasons":["REPLACEMENT"]}]}', 'solver-current', 'model-current', 'plan-shortfall');
		INSERT INTO operator_approvals (approval_id, event_id, plan_version, decision, decided_by, decided_at, rationale, correlation_id)
		VALUES ('approval-plan-shortfall', 'event-plan-shortfall', 1, 'APPROVED', 'operator', now(), 'Approved initial plan', 'plan-shortfall')`)
	if err != nil {
		t.Fatal(err)
	}
	report, err := NewPostgresReportSource(pool).EventReportData(context.Background(), "event-plan-shortfall")
	if err != nil {
		t.Fatal(err)
	}
	if report.Versions.Solver != "solver-current" {
		t.Fatal("selected plan control missing")
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
	if err := json.Unmarshal(body["planned_shortfall"], &intervals); err != nil || len(intervals) != 1 || string(intervals[0]["shortfall_kw"]) != "200" || string(intervals[0]["feasible_kw"]) != "800" {
		t.Fatalf("stored planned shortfall = %s", encoded)
	}
}
