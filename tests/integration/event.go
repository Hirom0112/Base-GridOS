package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
	"google.golang.org/protobuf/types/known/timestamppb"
	_ "modernc.org/sqlite"
)

const (
	telemetryBatch = 250
	statePoll      = 100 * time.Millisecond
	stateTimeout   = 2 * time.Minute
)

type commandState struct {
	deviceID   string
	generation int64
	state      string
}

type planExclusion struct {
	DeviceID string `json:"device_id"`
	Reason   string `json:"reason"`
}

func (stack *stack) publishTelemetry(t *testing.T, ctx context.Context, devices []FleetDevice, now time.Time, stateOfEnergy func(FleetDevice) float64) {
	t.Helper()
	client := gridosv1connect.NewTelemetryServiceClient(stack.client, stack.controlURL)
	for start := 0; start < len(devices); start += telemetryBatch {
		observations := make([]*gridosv1.TelemetryObservation, 0, telemetryBatch)
		for _, device := range devices[start:min(start+telemetryBatch, len(devices))] {
			observations = append(observations, &gridosv1.TelemetryObservation{
				ObservationId: fmt.Sprintf("%s-%s-%d", stack.scenario.Name, device.DeviceID, now.UnixNano()), DeviceId: device.DeviceID,
				Sequence: uint64(now.UnixNano()), ObservationTime: timestamppb.New(now), ValueState: gridosv1.ValueState_VALUE_STATE_PRESENT,
				StateOfEnergyPercent: stateOfEnergy(device),
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

func (stack *stack) boundary(t *testing.T) gridosv1.MeasurementBoundary {
	t.Helper()
	value, known := gridosv1.MeasurementBoundary_value["MEASUREMENT_BOUNDARY_"+stack.scenario.Event.Boundary]
	if !known {
		t.Fatalf("scenario boundary %q is not a contract boundary", stack.scenario.Event.Boundary)
	}
	return gridosv1.MeasurementBoundary(value)
}

func (stack *stack) createEvent(t *testing.T, ctx context.Context, eventID string) *gridosv1.DispatchEvent {
	t.Helper()
	request := connect.NewRequest(&gridosv1.CreateEventRequestRequest{EventRequest: &gridosv1.EventRequest{
		RequestId: eventID, EventType: "GRID_SERVICE", BeginTime: timestamppb.New(stack.scenario.Event.StartAt), EndTime: timestamppb.New(stack.scenario.Event.EndAt),
		TargetKw: stack.scenario.Event.TargetMW * 1000, MeasurementBoundary: stack.boundary(t),
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

func (stack *stack) waitEventState(t *testing.T, ctx context.Context, eventID, wanted string) *gridosv1.GetEventResponse {
	t.Helper()
	target, known := gridosv1.DispatchEventState_value["DISPATCH_EVENT_STATE_"+wanted]
	if !known {
		t.Fatalf("%q is not a dispatch event state", wanted)
	}
	deadline := time.Now().Add(stateTimeout)
	var response *gridosv1.GetEventResponse
	for time.Now().Before(deadline) {
		if response = stack.getEvent(t, ctx, eventID); int32(response.GetEvent().GetState()) >= target {
			return response
		}
		time.Sleep(statePoll)
	}
	t.Fatalf("event %s is %s, not %s, after %s\nworker log:\n%s", eventID, response.GetEvent().GetState(), wanted, stateTimeout, stack.logTail(t, "worker"))
	return nil
}

func (stack *stack) commandStates(t *testing.T, ctx context.Context, eventID string) map[string]commandState {
	t.Helper()
	rows, err := stack.pool.Query(ctx, `SELECT DISTINCT ON (intent.command_id) intent.command_id, intent.device_id, intent.generation, state.state
		FROM command_intents AS intent JOIN command_states AS state USING (command_id)
		WHERE intent.event_id = $1 ORDER BY intent.command_id, state.recorded_at DESC`, eventID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	states := make(map[string]commandState)
	for rows.Next() {
		var commandID string
		var command commandState
		if err = rows.Scan(&commandID, &command.deviceID, &command.generation, &command.state); err != nil {
			t.Fatal(err)
		}
		states[commandID] = command
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	return states
}

func (stack *stack) planExclusions(t *testing.T, ctx context.Context, eventID string) map[string]string {
	t.Helper()
	var contents []byte
	err := stack.pool.QueryRow(ctx, "SELECT exclusions FROM eligibility_snapshots WHERE event_id = $1 ORDER BY captured_at DESC LIMIT 1", eventID).Scan(&contents)
	if err != nil {
		t.Fatal(err)
	}
	var exclusions []planExclusion
	if err = json.Unmarshal(contents, &exclusions); err != nil {
		t.Fatal(err)
	}
	reasons := make(map[string]string, len(exclusions))
	for _, exclusion := range exclusions {
		reasons[exclusion.DeviceID] = exclusion.Reason
	}
	return reasons
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
