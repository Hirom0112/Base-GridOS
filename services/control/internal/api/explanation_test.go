package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type explanationStore struct {
	EventStore
	request *gridosv1.OptimizationRequest
	plan    *gridosv1.DispatchPlan
}

func (store explanationStore) LoadPlan(_ context.Context, _ string, _ uint64) (*gridosv1.OptimizationRequest, *gridosv1.DispatchPlan, error) {
	return store.request, store.plan, nil
}

func TestGetPlanExplanation(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	events := NewMemoryEventStore()
	events.Put(&gridosv1.DispatchEvent{EventId: "event-explain", PlanVersion: 7, State: gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_VALIDATED})
	store := explanationStore{
		EventStore: events,
		request: &gridosv1.OptimizationRequest{Devices: []*gridosv1.DeviceState{
			{DeviceId: "a", EffectiveReserveKwh: 8},
			{DeviceId: "b", EffectiveReserveKwh: 12},
		}},
		plan: &gridosv1.DispatchPlan{
			EventId: "event-explain", PlanVersion: 7,
			ObjectiveBreakdown: &gridosv1.ObjectiveBreakdown{GridValue: 50, DegradationCost: 3},
			ConstraintMargins:  []*gridosv1.ConstraintMargin{{ConstraintName: "feeder", Margin: 4, Units: "kW"}},
			Exclusions:         []*gridosv1.DeviceExclusion{{DeviceId: "c", Reason: gridosv1.ExclusionReason_EXCLUSION_REASON_RESERVE}},
			Shortfalls: []*gridosv1.ShortfallReport{{
				IntervalBeginTime: timestamppb.New(now), IntervalEndTime: timestamppb.New(now.Add(5 * time.Minute)),
				RequestedKw: 100, FeasibleKw: 90, ShortfallKw: 10,
			}},
		},
	}
	service := NewService(store, nil, nil, func() time.Time { return now })
	server := httptest.NewServer(NewHandler(service))
	defer server.Close()
	client := gridosv1connect.NewDispatchServiceClient(http.DefaultClient, server.URL)
	request := connect.NewRequest(&gridosv1.GetPlanExplanationRequest{EventId: "event-explain", PlanVersion: 7})
	request.Header().Set(roleHeader, "analyst")
	response, err := client.GetPlanExplanation(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	explanation := response.Msg
	if explanation.GetObjectiveBreakdown().GetGridValue() != 50 || explanation.GetReserveHeldBackKwh() != 20 || explanation.GetConstraintMargins()[0].GetMargin() != 4 || explanation.GetExclusions()[0].GetDeviceId() != "c" || explanation.GetShortfalls()[0].GetShortfallKw() != 10 {
		t.Fatalf("explanation = %#v", explanation)
	}
	request.Header().Set(roleHeader, "member")
	_, err = client.GetPlanExplanation(context.Background(), request)
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("member explanation code = %v", connect.CodeOf(err))
	}
	request.Header().Set(roleHeader, "analyst")
	request.Msg.PlanVersion = 6
	_, err = client.GetPlanExplanation(context.Background(), request)
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("stale version code = %v", connect.CodeOf(err))
	}
}
