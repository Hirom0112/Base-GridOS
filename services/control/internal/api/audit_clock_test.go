package api

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet"
)

func TestApprovalAndLaunchStampTheServerClockNotTheClientClock(t *testing.T) {
	serverNow := time.Date(2026, 9, 27, 14, 58, 1, 371000000, time.UTC)
	clientEarlier := serverNow.Add(-16 * time.Millisecond)
	store := NewMemoryEventStore()
	store.Put(&gridosv1.DispatchEvent{EventId: "event-1", State: gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_VALIDATED, PlanVersion: 3, CreatedAt: timestamp(serverNow), UpdatedAt: timestamp(serverNow)})
	service := NewService(store, fleet.NewTwin(time.Minute), nil, func() time.Time { return serverNow })
	service.approveWorkflow = func(context.Context, string, dispatchWorkflowApproval) error { return nil }
	var signaled *gridosv1.LaunchEventRequest
	service.launchWorkflow = func(_ context.Context, _ string, launch *gridosv1.LaunchEventRequest) error {
		signaled = launch
		return nil
	}

	approval := connect.NewRequest(&gridosv1.ApproveEventRequest{EventId: "event-1", PlanVersion: 3, IdempotencyKey: "approve-1", ApprovedBy: "approver-1", ApprovedAt: timestamp(clientEarlier)})
	approval.Header().Set(roleHeader, "approver")
	if _, err := service.ApproveEvent(context.Background(), approval); err != nil {
		t.Fatal(err)
	}
	audit := store.Audit()
	if len(audit) != 1 || audit[0].Action != "EVENT_APPROVED" || !audit[0].OccurredAt.Equal(serverNow) {
		t.Fatalf("approval audit = %#v, want EVENT_APPROVED at server clock %s", audit, serverNow)
	}

	launch := connect.NewRequest(&gridosv1.LaunchEventRequest{EventId: "event-1", PlanVersion: 3, IdempotencyKey: "launch-1", RequestedBy: "approver-1", RequestedAt: timestamp(clientEarlier)})
	launch.Header().Set(roleHeader, "approver")
	if _, err := service.LaunchEvent(context.Background(), launch); err != nil {
		t.Fatal(err)
	}
	if signaled == nil || !signaled.GetRequestedAt().AsTime().Equal(serverNow) {
		t.Fatalf("launch signal = %#v, want requested_at at server clock %s", signaled, serverNow)
	}
	if !launch.Msg.GetRequestedAt().AsTime().Equal(clientEarlier) {
		t.Fatalf("launch request mutated: %s", launch.Msg.GetRequestedAt().AsTime())
	}
}
