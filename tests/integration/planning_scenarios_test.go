package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestHeatEventCanonical(t *testing.T) {
	stack := startStack(t, "heat-event-canonical")
	ctx := context.Background()
	now := time.Now().UTC()
	stack.publishTelemetry(t, ctx, stack.fleetDevices(t, stack.scenario.Fleet.Size), now, constantStateOfEnergy)
	eventID := fmt.Sprintf("heat-canonical-%d", now.UnixNano())
	response := stack.runEvent(t, ctx, eventID, now)
	client := gridosv1connect.NewEventsServiceClient(stack.client, stack.controlURL)
	request := connect.NewRequest(&gridosv1.GetEventTimelineRequest{EventId: eventID})
	request.Header().Set("X-GridOS-Role", "operator")
	timeline, err := client.GetEventTimeline(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	missing := false
	uncertain := false
	recovery := make(map[gridosv1.EventExceptionKind]bool)
	for _, exception := range timeline.Msg.GetExceptions() {
		if exception.GetEventId() != eventID || exception.GetOccurredAt() == nil || exception.GetEvidenceId() == "" {
			t.Fatalf("incomplete event exception: %v", exception)
		}
		missing = missing || exception.GetKind() == gridosv1.EventExceptionKind_EVENT_EXCEPTION_KIND_MISSING_TELEMETRY
		uncertain = uncertain || exception.GetKind() == gridosv1.EventExceptionKind_EVENT_EXCEPTION_KIND_UNCERTAIN_COMMAND
		recovery[exception.GetKind()] = true
	}
	if !missing {
		t.Fatal("heat event has no MISSING telemetry exception")
	}
	if !uncertain {
		t.Fatal("heat event has no UNCERTAIN command exception")
	}
	actions := map[string]gridosv1.EventExceptionKind{
		"RETRY":                 gridosv1.EventExceptionKind_EVENT_EXCEPTION_KIND_COMMAND_RETRY,
		"REMOVE_STALE_CAPACITY": gridosv1.EventExceptionKind_EVENT_EXCEPTION_KIND_STALE_CAPACITY_REMOVED,
		"REBALANCE":             gridosv1.EventExceptionKind_EVENT_EXCEPTION_KIND_REBALANCED_COMMAND,
	}
	for _, action := range stack.scenario.Expected.RequiredRecoveryActions {
		kind, known := actions[action]
		if !known || !recovery[kind] {
			t.Fatalf("required recovery action %q missing from event exceptions", action)
		}
	}
	watchRequest := connect.NewRequest(&gridosv1.WatchEventRequest{EventId: eventID})
	watchRequest.Header().Set("X-GridOS-Role", "operator")
	watch, err := client.WatchEvent(ctx, watchRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = watch.Close() }()
	if !watch.Receive() {
		t.Fatal(watch.Err())
	}
	if len(watch.Msg().GetExceptions()) != len(timeline.Msg.GetExceptions()) {
		t.Fatalf("watch exceptions = %d, timeline exceptions = %d", len(watch.Msg().GetExceptions()), len(timeline.Msg.GetExceptions()))
	}
	stack.assertOutcome(t, ctx, eventID, response)
}

func TestInfeasibleTargetShortfall(t *testing.T) {
	stack := startStack(t, "infeasible-target-shortfall")
	ctx := context.Background()
	now := time.Now().UTC()
	stack.publishTelemetry(t, ctx, stack.cohort(t), now, constantStateOfEnergy)
	eventID := fmt.Sprintf("infeasible-shortfall-%d", now.UnixNano())
	response := stack.runEvent(t, ctx, eventID, now)
	plan := stack.storedPlan(t, ctx, eventID)
	if len(plan.GetShortfalls()) == 0 {
		t.Fatal("infeasible target has no interval shortfall")
	}
	for _, interval := range plan.GetShortfalls() {
		if interval.GetShortfallKw() <= 0 || interval.GetShortfallKw() > interval.GetRequestedKw() {
			t.Fatalf("invalid quantified shortfall: %#v", interval)
		}
	}
	stack.assertOutcome(t, ctx, eventID, response)
}

func TestOptimizerTimeoutFallback(t *testing.T) {
	stack := startStack(t, "optimizer-timeout-fallback")
	ctx := context.Background()
	now := time.Now().UTC()
	stack.publishTelemetry(t, ctx, stack.cohort(t), now, constantStateOfEnergy)
	eventID := fmt.Sprintf("optimizer-timeout-%d", now.UnixNano())
	response := stack.runEvent(t, ctx, eventID, now)
	plan := stack.storedPlan(t, ctx, eventID)
	if !plan.GetFallbackUsed() || plan.GetFallbackReason() != "TIMEOUT" {
		t.Fatalf("timeout plan fallback = %t, reason = %q", plan.GetFallbackUsed(), plan.GetFallbackReason())
	}
	var auditCount int
	if err := stack.pool.QueryRow(ctx, `SELECT count(*) FROM audit_journal WHERE resource_id = $1 AND action = 'PLAN_FALLBACK_SELECTED' AND new_values->>'fallback_reason' = 'TIMEOUT'`, eventID).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 {
		t.Fatalf("timeout fallback audit rows = %d, want 1", auditCount)
	}
	stack.assertOutcome(t, ctx, eventID, response)
}

func (stack *stack) storedPlan(t *testing.T, ctx context.Context, eventID string) *gridosv1.DispatchPlan {
	t.Helper()
	var encoded []byte
	if err := stack.pool.QueryRow(ctx, `SELECT plan FROM plan_versions WHERE event_id = $1 AND version = 1`, eventID).Scan(&encoded); err != nil {
		t.Fatal(err)
	}
	plan := new(gridosv1.DispatchPlan)
	if err := protojson.Unmarshal(encoded, plan); err != nil {
		t.Fatal(err)
	}
	return plan
}
