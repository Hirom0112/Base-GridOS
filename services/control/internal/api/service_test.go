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
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet"
)

func TestFleetServiceAuthorizationAndLocationPrivacy(t *testing.T) {
	server, fleetClient, _, _ := testServer(t)
	defer server.Close()
	roles := map[string]bool{"operator": true, "approver": true, "analyst": true, "partner": true, "service": true, "member": false}
	for role, allowed := range roles {
		request := connect.NewRequest(&gridosv1.GetFleetSummaryRequest{})
		request.Header().Set(roleHeader, role)
		response, err := fleetClient.GetFleetSummary(context.Background(), request)
		if allowed && (err != nil || response.Msg.GetSummary().GetDispatchableNowMw().GetValue() != 0.005) {
			t.Fatalf("role %q summary = %#v, %v", role, response, err)
		}
		if !allowed && connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Fatalf("role %q code = %v, want permission denied", role, connect.CodeOf(err))
		}
	}
	aggregateRequest := connect.NewRequest(&gridosv1.ListSitesRequest{})
	aggregateRequest.Header().Set(roleHeader, "operator")
	aggregate, err := fleetClient.ListSites(context.Background(), aggregateRequest)
	if err != nil || len(aggregate.Msg.GetSites()) != 0 {
		t.Fatalf("aggregate sites = %#v, %v", aggregate, err)
	}
	exactRequest := connect.NewRequest(&gridosv1.ListSitesRequest{RequestExactH3Cells: true})
	exactRequest.Header().Set(roleHeader, "operator")
	_, err = fleetClient.ListSites(context.Background(), exactRequest)
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("exact without permission code = %v", connect.CodeOf(err))
	}
	exactRequest.Header().Set(permissionsHeader, siteLocation)
	exact, err := fleetClient.ListSites(context.Background(), exactRequest)
	if err != nil || exact.Msg.GetSites()[0].GetExact().GetSite().GetSiteId() != "site-1" {
		t.Fatalf("exact sites = %#v, %v", exact, err)
	}
}

func TestDispatchServiceApprovalLaunchAndGroupedExclusions(t *testing.T) {
	server, _, dispatchClient, store := testServer(t)
	defer server.Close()
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	store.Put(&gridosv1.DispatchEvent{EventId: "event-1", State: gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_VALIDATED, PlanVersion: 3, CreatedAt: timestamp(now), UpdatedAt: timestamp(now)})
	store.SetExclusions("event-1", map[gridosv1.ExclusionReason]uint64{
		gridosv1.ExclusionReason_EXCLUSION_REASON_RESERVE:         12,
		gridosv1.ExclusionReason_EXCLUSION_REASON_STALE_TELEMETRY: 3,
	})

	wrongRole := connect.NewRequest(&gridosv1.ApproveEventRequest{EventId: "event-1", PlanVersion: 3, IdempotencyKey: "approve-1", ApprovedBy: "operator-1", ApprovedAt: timestamp(now)})
	wrongRole.Header().Set(roleHeader, "operator")
	_, err := dispatchClient.ApproveEvent(context.Background(), wrongRole)
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("wrong-role approval code = %v", connect.CodeOf(err))
	}

	launch := connect.NewRequest(&gridosv1.LaunchEventRequest{EventId: "event-1", PlanVersion: 3, IdempotencyKey: "launch-1", RequestedBy: "approver-1", RequestedAt: timestamp(now)})
	launch.Header().Set(roleHeader, "approver")
	_, err = dispatchClient.LaunchEvent(context.Background(), launch)
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("launch before approval code = %v", connect.CodeOf(err))
	}

	approval := connect.NewRequest(&gridosv1.ApproveEventRequest{EventId: "event-1", PlanVersion: 3, IdempotencyKey: "approve-1", ApprovedBy: "approver-1", ApprovedAt: timestamp(now)})
	approval.Header().Set(roleHeader, "approver")
	if _, err = dispatchClient.ApproveEvent(context.Background(), approval); err != nil {
		t.Fatal(err)
	}
	launched, err := dispatchClient.LaunchEvent(context.Background(), launch)
	if err != nil {
		t.Fatal(err)
	}
	if launched.Msg.GetEvent().GetState() != gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_APPROVED {
		t.Fatalf("launched event = %#v", launched.Msg.GetEvent())
	}
	audit := store.Audit()
	if len(audit) != 1 || audit[0].Action != "EVENT_APPROVED" {
		t.Fatalf("audit = %#v", audit)
	}

	get := connect.NewRequest(&gridosv1.GetEventRequest{EventId: "event-1"})
	get.Header().Set(roleHeader, "analyst")
	event, err := dispatchClient.GetEvent(context.Background(), get)
	if err != nil || len(event.Msg.GetExclusions()) != 2 || event.Msg.GetExclusions()[0].GetCount() != 12 || event.Msg.GetExclusions()[1].GetCount() != 3 {
		t.Fatalf("event exclusions = %#v, %v", event, err)
	}
}

func TestDispatchServiceCreatesEventIdempotently(t *testing.T) {
	server, _, dispatchClient, _ := testServer(t)
	defer server.Close()
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	message := &gridosv1.CreateEventRequestRequest{
		EventRequest:   &gridosv1.EventRequest{RequestId: "request-1", BeginTime: timestamp(now), EndTime: timestamp(now.Add(time.Hour)), TargetKw: 20000},
		IdempotencyKey: "create-1",
	}
	request := connect.NewRequest(message)
	request.Header().Set(roleHeader, "operator")
	first, err := dispatchClient.CreateEventRequest(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	retry := connect.NewRequest(message)
	retry.Header().Set(roleHeader, "operator")
	second, err := dispatchClient.CreateEventRequest(context.Background(), retry)
	if err != nil || first.Msg.GetEvent().GetEventId() != second.Msg.GetEvent().GetEventId() {
		t.Fatalf("idempotent events = %#v, %#v, %v", first, second, err)
	}
}

func testServer(t *testing.T) (*httptest.Server, gridosv1connect.FleetServiceClient, gridosv1connect.DispatchServiceClient, *MemoryEventStore) {
	t.Helper()
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	twin := fleet.NewTwin(time.Minute)
	twin.Accept(fleet.SiteState{SiteID: "site-1", ObservedAt: now, OperatingState: fleet.OnGrid, DispatchableKW: 5, DispatchableKWh: 10, Provenance: "simulated"})
	sites := []*gridosv1.AuthorizedSite{{
		Site:    &gridosv1.Site{SiteId: "site-1", LoadZone: "LZ_AEN", H3Cell: "87283472bffffff"},
		Devices: []*gridosv1.Device{{DeviceId: "device-1", SiteId: "site-1", BatteryParameters: &gridosv1.BatteryParameters{UsableEnergyKwh: 39.2, MaxDischargeKw: 10}}},
	}}
	store := NewMemoryEventStore()
	service := NewService(store, twin, sites, func() time.Time { return now })
	service.approveWorkflow = func(context.Context, string, dispatchWorkflowApproval) error { return nil }
	service.launchWorkflow = func(context.Context, string, *gridosv1.LaunchEventRequest) error { return nil }
	server := httptest.NewServer(NewHandler(service))
	return server, gridosv1connect.NewFleetServiceClient(http.DefaultClient, server.URL), gridosv1connect.NewDispatchServiceClient(http.DefaultClient, server.URL), store
}
