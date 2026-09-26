package endtoend

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/timestamppb"
	_ "modernc.org/sqlite"
)

const (
	controlURL  = "http://localhost:28080"
	gatewayURL  = "http://localhost:28081"
	gatewayAuth = "Bearer local-gateway"
)

type fleetDevice struct {
	DeviceID string `json:"device_id"`
}

func TestVerticalSlice(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	deviceID := firstFleetDevice(t)
	publishTelemetry(t, ctx, deviceID, now)
	assertFleetTelemetry(t, ctx)

	eventID := fmt.Sprintf("vertical-%d", now.UnixNano())
	dispatch := gridosv1connect.NewDispatchServiceClient(http.DefaultClient, controlURL)
	created := createEvent(t, ctx, dispatch, eventID, now)
	if created.GetState() != gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_VALIDATED || created.GetPlanVersion() != 1 {
		t.Fatalf("created event state = %s plan %d", created.GetState(), created.GetPlanVersion())
	}
	approveEvent(t, ctx, dispatch, eventID, now)
	launched := launchEvent(t, ctx, dispatch, eventID, now)
	if launched.GetState() != gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_ACKNOWLEDGED_OR_UNCERTAIN {
		t.Fatalf("launched event state = %s", launched.GetState())
	}
	response := getEvent(t, ctx, dispatch, eventID)
	report := response.GetReport()
	if report.GetRequestedMw() <= 0 || report.GetApprovedMw() <= 0 || report.GetCommandedMw() <= 0 || report.GetAcknowledgedMw() <= 0 {
		t.Fatalf("report power = %#v", report)
	}
	if len(report.GetProvenance()) == 0 || report.GetPolicyVersion() == "" || report.GetSolverVersion() == "" || report.GetModelVersion() == "" {
		t.Fatalf("report metadata = %#v", report)
	}
}

func TestDuplicateDelivery(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	identifier := fmt.Sprintf("duplicate-%d", now.UnixNano())
	pool := database(t, ctx)
	seedCommandEvent(t, ctx, pool, identifier, now)
	command := storage.CommandIntent{
		CommandID: identifier, IdempotencyKey: identifier, DeviceID: firstFleetDevice(t), EventID: identifier,
		PlanVersion: 1, Generation: 1, SetpointKW: 1, IssuedAt: now, EffectiveAt: now.Add(-time.Second),
		ExpiresAt: now.Add(time.Hour), PolicyVersion: "fleet-file", CorrelationID: identifier,
	}
	if err := storage.InsertCommand(ctx, pool, command); err != nil {
		t.Fatal(err)
	}
	claimed, err := storage.ClaimOutbox(ctx, pool, storage.OutboxClaim{AvailableAt: now, LeaseUntil: now.Add(time.Minute), BatchSize: 1})
	if err != nil || len(claimed) != 1 {
		t.Fatalf("claimed commands = %#v, %v", claimed, err)
	}
	if _, err = storage.TransitionCommand(ctx, pool, storage.CommandTransition{CommandID: identifier, ExpectedState: "PERSISTED", NextState: "SENT", OccurredAt: now, CorrelationID: identifier}); err != nil {
		t.Fatal(err)
	}
	client := gridosv1connect.NewCommandServiceClient(h2Client(), gatewayURL, connect.WithGRPC())
	request := connect.NewRequest(&gridosv1.SubmitCommandRequest{CommandIntent: commandMessage(claimed[0])})
	request.Header().Set("Authorization", gatewayAuth)
	first, err := client.SubmitCommand(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	retry := connect.NewRequest(&gridosv1.SubmitCommandRequest{CommandIntent: commandMessage(claimed[0])})
	retry.Header().Set("Authorization", gatewayAuth)
	second, err := client.SubmitCommand(ctx, retry)
	if err != nil || second.Msg.GetAcknowledgement().GetReceiptStatus() != gridosv1.CommandReceiptStatus_COMMAND_RECEIPT_STATUS_ACCEPTED {
		t.Fatalf("duplicate response = %#v, %v", second, err)
	}
	ack := first.Msg.GetAcknowledgement()
	if err = storage.RecordAcknowledgement(ctx, pool, storage.Acknowledgement{
		AcknowledgementID: ack.GetAcknowledgementId(), CommandID: identifier, IdempotencyKey: identifier,
		ReceiptStatus: "ACCEPTED", ReceivedAt: ack.GetReceivedAt().AsTime(), GatewayID: ack.GetGatewayId(), CorrelationID: identifier,
	}); err != nil {
		t.Fatal(err)
	}
	assertDeliveryCounts(t, ctx, pool, identifier)
}

func publishTelemetry(t *testing.T, ctx context.Context, deviceID string, now time.Time) {
	t.Helper()
	client := gridosv1connect.NewTelemetryServiceClient(http.DefaultClient, controlURL)
	request := connect.NewRequest(&gridosv1.PublishTelemetryRequest{GatewayId: "demo-gateway", Observations: []*gridosv1.TelemetryObservation{{
		ObservationId: "observation-" + deviceID, DeviceId: deviceID, Sequence: uint64(now.UnixNano()),
		ObservationTime: timestamppb.New(now), ValueState: gridosv1.ValueState_VALUE_STATE_PRESENT, StateOfEnergyPercent: 74,
		OperatingState: &gridosv1.TelemetryObservation_OnGrid{OnGrid: &gridosv1.OnGrid{ObservedAt: timestamppb.New(now), EstimatedBackupHoursAtCurrentUsage: 4, EstimatedBackupHoursAt_750Watts: 12}},
	}}})
	request.Header().Set("Authorization", gatewayAuth)
	response, err := client.PublishTelemetry(ctx, request)
	if err != nil || response.Msg.GetDurableReceiptId() == "" {
		t.Fatalf("telemetry receipt = %#v, %v", response, err)
	}
}

func assertFleetTelemetry(t *testing.T, ctx context.Context) {
	t.Helper()
	client := gridosv1connect.NewFleetServiceClient(http.DefaultClient, controlURL)
	request := connect.NewRequest(&gridosv1.GetFleetSummaryRequest{LoadZones: []string{"LZ_AEN"}})
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

func createEvent(t *testing.T, ctx context.Context, client gridosv1connect.DispatchServiceClient, eventID string, now time.Time) *gridosv1.DispatchEvent {
	t.Helper()
	begin := now.Add(time.Minute)
	request := connect.NewRequest(&gridosv1.CreateEventRequestRequest{EventRequest: &gridosv1.EventRequest{
		RequestId: eventID, EventType: "GRID_SERVICE", BeginTime: timestamppb.New(begin), EndTime: timestamppb.New(begin.Add(time.Hour)),
		TargetKw: 1, MeasurementBoundary: gridosv1.MeasurementBoundary_MEASUREMENT_BOUNDARY_METER_NET_EXPORT,
		LoadZones: []string{"LZ_AEN"}, CorrelationId: eventID,
	}, IdempotencyKey: "create-" + eventID})
	request.Header().Set("X-GridOS-Role", "operator")
	response, err := client.CreateEventRequest(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	return response.Msg.GetEvent()
}

func approveEvent(t *testing.T, ctx context.Context, client gridosv1connect.DispatchServiceClient, eventID string, now time.Time) {
	t.Helper()
	request := connect.NewRequest(&gridosv1.ApproveEventRequest{EventId: eventID, PlanVersion: 1, IdempotencyKey: "approve-" + eventID, ApprovedBy: "approver", ApprovedAt: timestamppb.New(now)})
	request.Header().Set("X-GridOS-Role", "approver")
	if _, err := client.ApproveEvent(ctx, request); err != nil {
		t.Fatal(err)
	}
}

func launchEvent(t *testing.T, ctx context.Context, client gridosv1connect.DispatchServiceClient, eventID string, now time.Time) *gridosv1.DispatchEvent {
	t.Helper()
	request := connect.NewRequest(&gridosv1.LaunchEventRequest{EventId: eventID, PlanVersion: 1, IdempotencyKey: "launch-" + eventID, RequestedBy: "approver", RequestedAt: timestamppb.New(now)})
	request.Header().Set("X-GridOS-Role", "approver")
	response, err := client.LaunchEvent(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	return response.Msg.GetEvent()
}

func getEvent(t *testing.T, ctx context.Context, client gridosv1connect.DispatchServiceClient, eventID string) *gridosv1.GetEventResponse {
	t.Helper()
	request := connect.NewRequest(&gridosv1.GetEventRequest{EventId: eventID})
	request.Header().Set("X-GridOS-Role", "operator")
	response, err := client.GetEvent(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	return response.Msg
}

func firstFleetDevice(t *testing.T) string {
	t.Helper()
	file, err := os.Open(filepath.Join(repositoryRoot(t), "testdata/fleets/austin-5000.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	var device fleetDevice
	if err = json.NewDecoder(bufio.NewReader(file)).Decode(&device); err != nil || device.DeviceID == "" {
		t.Fatalf("fleet device = %#v, %v", device, err)
	}
	return device.DeviceID
}

func database(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("GRIDOS_DATABASE_URL")
	if url == "" {
		url = "postgres://gridos:gridos@localhost:5432/gridos?sslmode=disable"
	}
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func seedCommandEvent(t *testing.T, ctx context.Context, pool *pgxpool.Pool, identifier string, now time.Time) {
	t.Helper()
	_, err := pool.Exec(ctx, `INSERT INTO dispatch_requests (request_id, event_type, begin_time, end_time, target_kw, measurement_boundary, load_zones, correlation_id)
		VALUES ($1, 'GRID_SERVICE', $2, $3, 1, 'METER_NET_EXPORT', ARRAY['LZ_AEN'], $1);
		INSERT INTO dispatch_events (event_id, request_id, state, plan_version, correlation_id) VALUES ($1, $1, 'APPROVED', 1, $1)`, identifier, now, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
}

func commandMessage(command storage.ClaimedCommand) *gridosv1.CommandIntent {
	return &gridosv1.CommandIntent{
		CommandId: command.CommandID, IdempotencyKey: command.IdempotencyKey, DeviceId: command.DeviceID,
		EventId: command.EventID, PlanVersion: uint64(command.PlanVersion), Generation: uint64(command.Generation),
		SetpointKw: command.SetpointKW, IssuedAt: timestamppb.New(command.IssuedAt), EffectiveAt: timestamppb.New(command.EffectiveAt),
		ExpiresAt: timestamppb.New(command.ExpiresAt), PolicyVersion: command.PolicyVersion, CorrelationId: command.CorrelationID,
	}
}

func assertDeliveryCounts(t *testing.T, ctx context.Context, pool *pgxpool.Pool, commandID string) {
	t.Helper()
	var acknowledgements int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM command_acknowledgements WHERE command_id = $1", commandID).Scan(&acknowledgements); err != nil {
		t.Fatal(err)
	}
	database, err := sql.Open("sqlite", filepath.Join(repositoryRoot(t), ".local/demo/gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()
	var effects int
	if err = database.QueryRowContext(ctx, "SELECT count(*) FROM commands WHERE command_id = ?", commandID).Scan(&effects); err != nil {
		t.Fatal(err)
	}
	if acknowledgements != 1 || effects != 1 {
		t.Fatalf("acknowledgements, physical effects = %d, %d", acknowledgements, effects)
	}
}

func h2Client() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	protocols := new(http.Protocols)
	protocols.SetUnencryptedHTTP2(true)
	transport.Protocols = protocols
	return &http.Client{Transport: transport, Timeout: 10 * time.Second}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate repository")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
}
