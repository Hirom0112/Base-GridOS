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
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestUnsafeAlternativeRejectsReserveBreachWithoutCommands(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	begin := now.Add(time.Minute)
	end := begin.Add(5 * time.Minute)
	events := NewMemoryEventStore()
	events.Put(&gridosv1.DispatchEvent{EventId: "event-alternative", PlanVersion: 7, State: gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_VALIDATED})
	input := &gridosv1.OptimizationRequest{
		EventId: "event-alternative", PlanVersion: 7, RequestedAt: timestamppb.New(now),
		ReservePolicy: &gridosv1.ReservePolicy{PolicyVersion: "policy-7"},
		Devices: []*gridosv1.DeviceState{{
			DeviceId: "device-a", UsableEnergyKwh: 10, EnergyKwh: 5.05, HardwareFloorKwh: 5,
			EffectiveReserveKwh: 5, MaxDischargeKw: 2, DischargeEfficiency: 1, ChargeEfficiency: 1,
			AvailabilityProbability: 1, TelemetryObservedAt: timestamppb.New(now),
		}},
	}
	safeEnergy := 5.05 - 0.5/12
	plan := &gridosv1.DispatchPlan{
		EventId: "event-alternative", PlanVersion: 7, CreatedAt: timestamppb.New(now),
		DeviceSchedules: []*gridosv1.DeviceSchedule{{
			DeviceId: "device-a", Intervals: []*gridosv1.DeviceScheduleInterval{{
				BeginTime: timestamppb.New(begin), EndTime: timestamppb.New(end),
				SetpointKw: 0.5, ExpectedEnergyKwh: safeEnergy,
			}},
		}},
	}
	service := NewService(explanationStore{EventStore: events, request: input, plan: plan}, nil, nil, func() time.Time { return now })
	server := httptest.NewServer(NewHandler(service))
	defer server.Close()
	client := gridosv1connect.NewDispatchServiceClient(http.DefaultClient, server.URL)
	request := connect.NewRequest(&gridosv1.ValidateUnsafeAlternativeRequest{
		EventId: "event-alternative", PlanVersion: 7, AlternativePlan: plan,
	})
	request.Header().Set(roleHeader, "approver")
	positive, err := client.ValidateUnsafeAlternative(context.Background(), request)
	if err != nil || !positive.Msg.GetApproved() || len(positive.Msg.GetViolations()) != 0 {
		t.Fatalf("safe alternative = %#v, %v", positive, err)
	}
	unsafe := proto.Clone(plan).(*gridosv1.DispatchPlan)
	unsafe.DeviceSchedules[0].Intervals[0].SetpointKw = 1
	unsafe.DeviceSchedules[0].Intervals[0].ExpectedEnergyKwh = 5.05 - 1.0/12
	request.Msg.AlternativePlan = unsafe
	rejected, err := client.ValidateUnsafeAlternative(context.Background(), request)
	if err != nil || rejected.Msg.GetApproved() || len(rejected.Msg.GetViolations()) == 0 || rejected.Msg.GetViolations()[0].GetCode() != "ENERGY_BELOW_RESERVE" {
		t.Fatalf("unsafe alternative = %#v, %v", rejected, err)
	}
	current, _, err := events.Get(context.Background(), "event-alternative")
	if err != nil || current.GetState() != gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_VALIDATED || len(events.Audit()) != 0 {
		t.Fatalf("validation changed event or audit: %#v, %v, %#v", current, err, events.Audit())
	}
	request.Header().Set(roleHeader, "member")
	_, err = client.ValidateUnsafeAlternative(context.Background(), request)
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("member validation code = %v", connect.CodeOf(err))
	}
}
