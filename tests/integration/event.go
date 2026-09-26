package integration

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
	"google.golang.org/protobuf/types/known/timestamppb"
	_ "modernc.org/sqlite"
)

const telemetryBatch = 250

func (stack *stack) publishTelemetry(t *testing.T, ctx context.Context, devices []FleetDevice, now time.Time, stateOfEnergyPercent float64) {
	t.Helper()
	client := gridosv1connect.NewTelemetryServiceClient(stack.client, stack.controlURL)
	for start := 0; start < len(devices); start += telemetryBatch {
		observations := make([]*gridosv1.TelemetryObservation, 0, telemetryBatch)
		for _, device := range devices[start:min(start+telemetryBatch, len(devices))] {
			observations = append(observations, &gridosv1.TelemetryObservation{
				ObservationId: fmt.Sprintf("%s-%s-%d", stack.scenario.Name, device.DeviceID, now.UnixNano()), DeviceId: device.DeviceID,
				Sequence: uint64(now.UnixNano()), ObservationTime: timestamppb.New(now), ValueState: gridosv1.ValueState_VALUE_STATE_PRESENT,
				StateOfEnergyPercent: stateOfEnergyPercent,
				OperatingState:       &gridosv1.TelemetryObservation_OnGrid{OnGrid: &gridosv1.OnGrid{ObservedAt: timestamppb.New(now), EstimatedBackupHoursAtCurrentUsage: 4, EstimatedBackupHoursAt_750Watts: 12}},
			})
		}
		request := connect.NewRequest(&gridosv1.PublishTelemetryRequest{GatewayId: gatewayID, Observations: observations})
		request.Header().Set("Authorization", gatewayToken)
		response, err := client.PublishTelemetry(ctx, request)
		if err != nil || response.Msg.GetDurableReceiptId() == "" {
			t.Fatalf("telemetry receipt = %#v, %v", response, err)
		}
	}
}

func (stack *stack) assertDispatchable(t *testing.T, ctx context.Context) {
	t.Helper()
	client := gridosv1connect.NewFleetServiceClient(stack.client, stack.controlURL)
	request := connect.NewRequest(&gridosv1.GetFleetSummaryRequest{LoadZones: []string{stack.scenario.Event.Region}})
	request.Header().Set("X-GridOS-Role", "operator")
	response, err := client.GetFleetSummary(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	aggregate := response.Msg.GetSummary().GetDispatchableNowMw()
	if aggregate.GetValue() <= 0 || len(aggregate.GetMetadata().GetProvenanceMix()) == 0 {
		t.Fatalf("dispatchable aggregate = %#v", aggregate)
	}
}

func (stack *stack) dispatch() gridosv1connect.DispatchServiceClient {
	return gridosv1connect.NewDispatchServiceClient(stack.client, stack.controlURL)
}

func (stack *stack) createEvent(t *testing.T, ctx context.Context, eventID string, now time.Time) *gridosv1.DispatchEvent {
	t.Helper()
	boundary, known := gridosv1.MeasurementBoundary_value["MEASUREMENT_BOUNDARY_"+stack.scenario.Event.Boundary]
	if !known {
		t.Fatalf("scenario boundary %q is not a contract boundary", stack.scenario.Event.Boundary)
	}
	begin := now.Add(time.Minute)
	request := connect.NewRequest(&gridosv1.CreateEventRequestRequest{EventRequest: &gridosv1.EventRequest{
		RequestId: eventID, EventType: "GRID_SERVICE", BeginTime: timestamppb.New(begin), EndTime: timestamppb.New(begin.Add(stack.scenario.duration())),
		TargetKw: stack.scenario.Event.TargetMW * 1000, MeasurementBoundary: gridosv1.MeasurementBoundary(boundary),
		LoadZones: []string{stack.scenario.Event.Region}, CorrelationId: eventID,
	}, IdempotencyKey: "create-" + eventID})
	request.Header().Set("X-GridOS-Role", "operator")
	response, err := stack.dispatch().CreateEventRequest(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	return response.Msg.GetEvent()
}

func (stack *stack) approveEvent(t *testing.T, ctx context.Context, eventID string, now time.Time) {
	t.Helper()
	request := connect.NewRequest(&gridosv1.ApproveEventRequest{EventId: eventID, PlanVersion: 1, IdempotencyKey: "approve-" + eventID, ApprovedBy: "approver", ApprovedAt: timestamppb.New(now)})
	request.Header().Set("X-GridOS-Role", "approver")
	if _, err := stack.dispatch().ApproveEvent(ctx, request); err != nil {
		t.Fatal(err)
	}
}

func (stack *stack) launchEvent(t *testing.T, ctx context.Context, eventID string, now time.Time) *gridosv1.DispatchEvent {
	t.Helper()
	request := connect.NewRequest(&gridosv1.LaunchEventRequest{EventId: eventID, PlanVersion: 1, IdempotencyKey: "launch-" + eventID, RequestedBy: "approver", RequestedAt: timestamppb.New(now)})
	request.Header().Set("X-GridOS-Role", "approver")
	response, err := stack.dispatch().LaunchEvent(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	return response.Msg.GetEvent()
}

func (stack *stack) getEvent(t *testing.T, ctx context.Context, eventID string) *gridosv1.GetEventResponse {
	t.Helper()
	request := connect.NewRequest(&gridosv1.GetEventRequest{EventId: eventID})
	request.Header().Set("X-GridOS-Role", "operator")
	response, err := stack.dispatch().GetEvent(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	return response.Msg
}

func (stack *stack) commandStates(t *testing.T, ctx context.Context, eventID string) map[string]string {
	t.Helper()
	rows, err := stack.pool.Query(ctx, `SELECT DISTINCT ON (intent.command_id) intent.command_id, state.state
		FROM command_intents AS intent JOIN command_states AS state USING (command_id)
		WHERE intent.event_id = $1 ORDER BY intent.command_id, state.recorded_at DESC`, eventID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	states := make(map[string]string)
	for rows.Next() {
		var commandID, state string
		if err = rows.Scan(&commandID, &state); err != nil {
			t.Fatal(err)
		}
		states[commandID] = state
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	return states
}

func (stack *stack) gatewayCommands(t *testing.T, ctx context.Context, eventID string) int {
	t.Helper()
	database, err := sql.Open("sqlite", stack.gatewayDB)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()
	var retained int
	if err = database.QueryRowContext(ctx, "SELECT count(*) FROM commands WHERE command_id LIKE ? || '-%'", eventID).Scan(&retained); err != nil {
		t.Fatal(err)
	}
	return retained
}
