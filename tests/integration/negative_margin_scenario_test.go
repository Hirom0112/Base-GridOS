package integration

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestNegativeMarginNoFlexDispatch(t *testing.T) {
	stack := startStack(t, "negative-margin-no-dispatch")
	ctx := context.Background()
	cohort := stack.cohort(t)
	index := slices.IndexFunc(cohort, func(device FleetDevice) bool { return device.ReservePreferencePercent <= 20 })
	if index < 0 {
		t.Fatal("cohort has no low-reserve member")
	}
	selected := cohort[index]
	stack.publishTelemetry(t, ctx, cohort, time.Now().UTC(), constantStateOfEnergy)
	memberID := stack.selectScenarioPlan(t, selected, 60, 10, 0)
	stack.scheduleTravelFlex(t, memberID, time.Now().UTC().Add(-time.Second), time.Now().UTC().Add(5*time.Minute), "negative-price")
	eventID := fmt.Sprintf("negative-price-%d", time.Now().UnixNano())
	response := stack.runEvent(t, ctx, eventID, time.Now().UTC())
	stack.assertOutcome(t, ctx, eventID, response)
	request := connect.NewRequest(&gridosv1.GetPlanExplanationRequest{EventId: eventID, PlanVersion: 1})
	request.Header().Set("X-GridOS-Role", "analyst")
	explanation, err := stack.dispatch().GetPlanExplanation(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	margin := explanation.Msg.GetMarginExplanation()
	if margin.GetConservativeMargin() >= 0 || len(margin.GetTerms()) == 0 || margin.GetTerms()[0].GetSource() != "FROZEN_SIMULATED_PRICE" {
		t.Fatalf("API negative price margin=%+v", margin)
	}
	var planJSON []byte
	if err := stack.pool.QueryRow(ctx, `SELECT plan FROM plan_versions WHERE event_id = $1 AND version = 1`, eventID).Scan(&planJSON); err != nil {
		t.Fatal(err)
	}
	var plan gridosv1.DispatchPlan
	if err := protojson.Unmarshal(planJSON, &plan); err != nil {
		t.Fatal(err)
	}
	for _, schedule := range plan.GetDeviceSchedules() {
		if schedule.GetDeviceId() == selected.DeviceID && schedule.GetReserveSelection() != gridosv1.ReserveSelection_RESERVE_SELECTION_BASE {
			t.Fatalf("negative price selected additional flex reserve=%s", schedule.GetReserveSelection())
		}
	}
	report := stack.scenarioReport(t, eventID)
	if report.Margin == nil || report.Margin.ValueUSD >= 0 || report.Margin.Bound != "UPPER" || report.Margin.PriceProvenance != "SIMULATED" || len(report.Margin.UnavailableCosts) != 5 {
		t.Fatalf("stored negative price margin=%+v", report.Margin)
	}
}
