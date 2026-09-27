package load

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type dispatchResult struct {
	persisted           int64
	sent                int64
	acknowledged        int64
	sentBeforePersisted int64
	readSamples         int
	readP95             time.Duration
}

func runDispatchPath(t *testing.T, deviceCount int) dispatchResult {
	t.Helper()
	stack := startLoadStack(t)
	devices := loadDeviceIDs(t, stack.root)
	if len(devices) != deviceCount {
		t.Fatalf("fleet has %d devices, want %d", len(devices), deviceCount)
	}
	telemetry := gridosv1connect.NewTelemetryServiceClient(&http.Client{Timeout: 15 * time.Second}, stack.controlURL)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	err := publishLoadTick(ctx, telemetry, devices, 1, time.Now().UTC(), 95)
	cancel()
	if err != nil {
		t.Fatalf("publish initial telemetry: %v", err)
	}
	stack.startDispatchServices(t)
	eventID := stack.name + "_dispatch"
	client := gridosv1connect.NewDispatchServiceClient(&http.Client{Timeout: 30 * time.Second}, stack.controlURL)
	createdAt := time.Now().UTC()
	begin, end := createdAt.Add(5*time.Minute), createdAt.Add(10*time.Minute)
	create := connect.NewRequest(&gridosv1.CreateEventRequestRequest{
		EventRequest: &gridosv1.EventRequest{RequestId: eventID, EventType: "GRID_SERVICE", BeginTime: timestamppb.New(begin), EndTime: timestamppb.New(end),
			TargetKw: 50000, MeasurementBoundary: gridosv1.MeasurementBoundary_MEASUREMENT_BOUNDARY_METER_NET_EXPORT,
			LoadZones: []string{"LZ_AEN"}, CorrelationId: eventID},
		IdempotencyKey: "create-" + eventID,
	})
	create.Header().Set("X-GridOS-Role", "operator")
	if _, err = client.CreateEventRequest(context.Background(), create); err != nil {
		t.Fatalf("create event: %v", err)
	}
	stack.waitDispatchState(t, eventID, "VALIDATED")
	var planned int
	if err = stack.database.QueryRow(context.Background(), `SELECT jsonb_array_length(plan->'deviceSchedules') FROM plan_versions WHERE event_id = $1 AND version = 1`, eventID).Scan(&planned); err != nil {
		t.Fatalf("read planned schedules: %v", err)
	}
	if planned != deviceCount {
		var exclusions []byte
		if err = stack.database.QueryRow(context.Background(), `SELECT plan->'exclusions' FROM plan_versions WHERE event_id = $1 AND version = 1`, eventID).Scan(&exclusions); err != nil {
			t.Fatal(err)
		}
		var reasons []struct {
			Reason string `json:"reason"`
		}
		if err = json.Unmarshal(exclusions, &reasons); err != nil {
			t.Fatal(err)
		}
		counts := make(map[string]int)
		for _, exclusion := range reasons {
			counts[exclusion.Reason]++
		}
		t.Fatalf("planned device schedules=%d, want %d, exclusions=%v", planned, deviceCount, counts)
	}
	approvedAt := time.Now().UTC()
	approve := connect.NewRequest(&gridosv1.ApproveEventRequest{EventId: eventID, PlanVersion: 1, IdempotencyKey: "approve-" + eventID,
		ApprovedBy: "load-approver", ApprovedAt: timestamppb.New(approvedAt)})
	approve.Header().Set("X-GridOS-Role", "approver")
	if _, err = client.ApproveEvent(context.Background(), approve); err != nil {
		t.Fatalf("approve event: %v", err)
	}
	readContext, stopReads := context.WithCancel(context.Background())
	readResults := make(chan time.Duration, 10000)
	readErrors := make(chan error, 1)
	go sampleFleetReads(readContext, stack.controlURL, readResults, readErrors)
	defer stopReads()
	launch := connect.NewRequest(&gridosv1.LaunchEventRequest{EventId: eventID, PlanVersion: 1, IdempotencyKey: "launch-" + eventID,
		RequestedBy: "load-approver", RequestedAt: timestamppb.New(approvedAt)})
	launch.Header().Set("X-GridOS-Role", "approver")
	if _, err = client.LaunchEvent(context.Background(), launch); err != nil {
		t.Fatalf("launch event: %v", err)
	}
	result := stack.waitForDispatchSend(t, eventID, deviceCount)
	for len(readResults) < 100 {
		select {
		case err = <-readErrors:
			t.Fatal(err)
		case <-time.After(100 * time.Millisecond):
		}
	}
	stopReads()
	var samples []time.Duration
	for sample := range readResults {
		if sample > 0 {
			samples = append(samples, sample)
		}
	}
	select {
	case err = <-readErrors:
		t.Fatal(err)
	default:
	}
	result.readSamples = len(samples)
	result.readP95 = readP95(samples)
	return result
}

func (stack *loadStack) startDispatchServices(t *testing.T) {
	t.Helper()
	decisionAddress, gatewayAddress := loadAddress(t), loadAddress(t)
	_, decisionPort, err := net.SplitHostPort(decisionAddress)
	if err != nil {
		t.Fatal(err)
	}
	stack.start(t, "decision", decisionAddress, "uv", nil, "run", "--project", "services/decision", "python", "-m", "gridos.server", "--port", decisionPort)
	binDir := t.TempDir()
	gateway := buildLoadBinary(t, stack.root, binDir, "gateway", "./services/gateway-simulator/cmd/gateway-simulator")
	worker := buildLoadBinary(t, stack.root, binDir, "worker", "./services/control/cmd/worker")
	stack.start(t, "gateway", gatewayAddress, gateway, []string{"GRIDOS_GATEWAY_TOKEN=" + loadGatewayToken},
		"--fleet", filepath.Join(stack.root, "testdata/fleets/austin-5000.jsonl"),
		"--scenario-start", time.Now().UTC().Format(time.RFC3339Nano), "--address", gatewayAddress,
		"--database", filepath.Join(stack.logDir, "gateway.db"), "--gateway-id", "load-gateway", "--cadence", "5s", "--control-address", stack.controlURL)
	stack.start(t, "worker", "", worker, []string{
		"GRIDOS_DATABASE_URL=" + stack.databaseURL, "GRIDOS_FLEET=" + filepath.Join(stack.root, "testdata/fleets/austin-5000.jsonl"),
		"GRIDOS_GATEWAY_ADDR=http://" + gatewayAddress, "GRIDOS_GATEWAY_TOKEN=" + loadGatewayToken,
		"GRIDOS_DECISION_ADDR=http://" + decisionAddress, "GRIDOS_TASK_QUEUE=" + stack.name,
		"GRIDOS_CODE_VERSION=load-test", "GRIDOS_REPLAY_DIR=" + filepath.Join(stack.logDir, "replay"),
	})
}

func (stack *loadStack) waitDispatchState(t *testing.T, eventID, wanted string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Minute)
	var state string
	for time.Now().Before(deadline) {
		if err := stack.database.QueryRow(context.Background(), `SELECT state FROM dispatch_events WHERE event_id = $1`, eventID).Scan(&state); err != nil {
			t.Fatal(err)
		}
		if state == wanted {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("event %s remained %s, did not reach %s", eventID, state, wanted)
}

func (stack *loadStack) waitForDispatchSend(t *testing.T, eventID string, wanted int) dispatchResult {
	t.Helper()
	deadline := time.Now().Add(5 * time.Minute)
	var result dispatchResult
	for time.Now().Before(deadline) {
		err := stack.database.QueryRow(context.Background(), `SELECT
			(SELECT count(*) FROM command_intents WHERE event_id = $1),
			(SELECT count(*) FROM command_states AS state JOIN command_intents AS intent USING (command_id)
			 WHERE intent.event_id = $1 AND state.state = 'SENT'),
			(SELECT count(*) FROM command_acknowledgements AS ack JOIN command_intents AS intent USING (command_id)
			 WHERE intent.event_id = $1 AND ack.receipt_status = 'ACCEPTED')`, eventID).Scan(&result.persisted, &result.sent, &result.acknowledged)
		if err != nil {
			t.Fatal(err)
		}
		if result.sent > 0 && result.acknowledged > 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if result.persisted != int64(wanted) || result.sent == 0 || result.acknowledged == 0 {
		t.Fatalf("event %s persisted=%d sent=%d acknowledged=%d, want %d persisted and a network receipt", eventID, result.persisted, result.sent, result.acknowledged, wanted)
	}
	err := stack.database.QueryRow(context.Background(), `SELECT count(*) FROM command_states AS persisted
		JOIN command_intents AS intent USING (command_id)
		WHERE intent.event_id = $1 AND persisted.state = 'PERSISTED' AND persisted.recorded_at > (
			SELECT min(sent.recorded_at) FROM command_states AS sent
			JOIN command_intents AS outgoing USING (command_id)
			WHERE outgoing.event_id = $1 AND sent.state = 'SENT')`, eventID).Scan(&result.sentBeforePersisted)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func sampleFleetReads(ctx context.Context, controlURL string, results chan<- time.Duration, failures chan<- error) {
	defer close(results)
	client := gridosv1connect.NewFleetServiceClient(&http.Client{Timeout: 2 * time.Second}, controlURL)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		request := connect.NewRequest(&gridosv1.GetFleetSummaryRequest{LoadZones: []string{"LZ_AEN"}})
		request.Header().Set("X-GridOS-Role", "operator")
		start := time.Now()
		response, err := client.GetFleetSummary(ctx, request)
		if err != nil {
			if ctx.Err() == nil {
				failures <- err
			}
			return
		}
		if response.Msg.GetSummary() == nil {
			failures <- fmt.Errorf("fleet summary response is empty")
			return
		}
		results <- time.Since(start)
	}
}

func readP95(samples []time.Duration) time.Duration {
	if len(samples) == 0 {
		return 0
	}
	slices.Sort(samples)
	index := (95*len(samples) + 99) / 100
	return samples[max(index-1, 0)]
}
