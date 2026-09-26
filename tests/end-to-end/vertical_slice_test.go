package endtoend

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/timestamppb"
	_ "modernc.org/sqlite"
)

const gatewayAuth = "Bearer local-gateway"

var (
	controlURL = serviceURL("GRIDOS_CONTROL_URL", "http://localhost:28080")
	gatewayURL = serviceURL("GRIDOS_GATEWAY_URL", "http://localhost:28081")
)

type fleetDevice struct {
	DeviceID string `json:"device_id"`
}

type commandIntent struct {
	CommandID      string
	IdempotencyKey string
	DeviceID       string
	EventID        string
	PlanVersion    int64
	Generation     int64
	SetpointKW     float64
	IssuedAt       time.Time
	EffectiveAt    time.Time
	ExpiresAt      time.Time
	PolicyVersion  string
	CorrelationID  string
}

func TestVerticalSlice(t *testing.T) {
	requireDemo(t)
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
	requireDemo(t)
	ctx := context.Background()
	now := time.Now().UTC()
	identifier := fmt.Sprintf("duplicate-%d", now.UnixNano())
	pool := database(t, ctx)
	deviceID := firstFleetDevice(t)
	publishTelemetry(t, ctx, deviceID, now)
	dispatch := gridosv1connect.NewDispatchServiceClient(http.DefaultClient, controlURL)
	createEvent(t, ctx, dispatch, identifier, now)
	approveEvent(t, ctx, dispatch, identifier, now)
	command := commandIntent{
		CommandID: identifier, IdempotencyKey: identifier, DeviceID: deviceID, EventID: identifier,
		PlanVersion: 1, Generation: 1, SetpointKW: 1, IssuedAt: now, EffectiveAt: now.Add(-time.Second),
		ExpiresAt: now.Add(time.Hour), PolicyVersion: "fleet-file", CorrelationID: identifier,
	}
	insertCommand(t, ctx, pool, command)
	claimed := claimCommand(t, ctx, pool, identifier, now)
	transitionCommand(t, ctx, pool, identifier, "PERSISTED", "SENT", now)
	client := gridosv1connect.NewCommandServiceClient(h2Client(), gatewayURL, connect.WithGRPC())
	request := connect.NewRequest(&gridosv1.SubmitCommandRequest{CommandIntent: commandMessage(claimed)})
	request.Header().Set("Authorization", gatewayAuth)
	first, err := client.SubmitCommand(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	retry := connect.NewRequest(&gridosv1.SubmitCommandRequest{CommandIntent: commandMessage(claimed)})
	retry.Header().Set("Authorization", gatewayAuth)
	second, err := client.SubmitCommand(ctx, retry)
	if err != nil || second.Msg.GetAcknowledgement().GetReceiptStatus() != gridosv1.CommandReceiptStatus_COMMAND_RECEIPT_STATUS_ACCEPTED {
		t.Fatalf("duplicate response = %#v, %v", second, err)
	}
	ack := first.Msg.GetAcknowledgement()
	recordAcknowledgement(t, ctx, pool, ack, identifier)
	assertDeliveryCounts(t, ctx, pool, identifier)
}

func requireDemo(t *testing.T) {
	t.Helper()
	for _, address := range []string{controlURL, gatewayURL} {
		address = address[len("http://"):]
		connection, err := net.DialTimeout("tcp", address, 250*time.Millisecond)
		if err != nil {
			t.Skip("demo stack is not listening; run make test-e2e")
		}
		if err = connection.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func serviceURL(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
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

func insertCommand(t *testing.T, ctx context.Context, pool *pgxpool.Pool, command commandIntent) {
	t.Helper()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `INSERT INTO command_intents
		(command_id, idempotency_key, device_id, event_id, plan_version, generation, setpoint_kw, issued_at, effective_at, expires_at, policy_version, correlation_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`, command.CommandID, command.IdempotencyKey, command.DeviceID, command.EventID, command.PlanVersion, command.Generation, command.SetpointKW, command.IssuedAt, command.EffectiveAt, command.ExpiresAt, command.PolicyVersion, command.CorrelationID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, "INSERT INTO command_outbox (command_id, state, correlation_id) VALUES ($1, 'PENDING', $2)", command.CommandID, command.CorrelationID); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, "INSERT INTO command_states (command_id, state, recorded_at, correlation_id) VALUES ($1, 'PERSISTED', $2, $3)", command.CommandID, command.IssuedAt, command.CorrelationID); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func claimCommand(t *testing.T, ctx context.Context, pool *pgxpool.Pool, commandID string, now time.Time) commandIntent {
	t.Helper()
	var command commandIntent
	err := pool.QueryRow(ctx, `UPDATE command_outbox SET state = 'PUBLISHING', attempts = attempts + 1, next_attempt_at = $2
		WHERE command_id = $1 AND state = 'PENDING' RETURNING command_id`, commandID, now.Add(time.Minute)).Scan(&command.CommandID)
	if err != nil {
		t.Fatal(err)
	}
	err = pool.QueryRow(ctx, `SELECT idempotency_key, device_id, event_id, plan_version, generation, setpoint_kw, issued_at, effective_at, expires_at, policy_version, correlation_id
		FROM command_intents WHERE command_id = $1`, commandID).Scan(&command.IdempotencyKey, &command.DeviceID, &command.EventID, &command.PlanVersion, &command.Generation, &command.SetpointKW, &command.IssuedAt, &command.EffectiveAt, &command.ExpiresAt, &command.PolicyVersion, &command.CorrelationID)
	if err != nil {
		t.Fatal(err)
	}
	return command
}

func transitionCommand(t *testing.T, ctx context.Context, pool *pgxpool.Pool, commandID, expected, next string, at time.Time) {
	t.Helper()
	tag, err := pool.Exec(ctx, `INSERT INTO command_states (command_id, state, recorded_at, correlation_id)
		SELECT $1, $3, GREATEST($4, recorded_at + interval '1 microsecond'), $1 FROM command_states
		WHERE command_id = $1 AND state = $2 ORDER BY recorded_at DESC LIMIT 1`, commandID, expected, next, at)
	if err != nil || tag.RowsAffected() != 1 {
		t.Fatalf("transition %s to %s: %v", expected, next, err)
	}
}

func recordAcknowledgement(t *testing.T, ctx context.Context, pool *pgxpool.Pool, acknowledgement *gridosv1.CommandAcknowledgement, commandID string) {
	t.Helper()
	_, err := pool.Exec(ctx, `INSERT INTO command_acknowledgements
		(acknowledgement_id, command_id, idempotency_key, receipt_status, received_at, gateway_id, correlation_id)
		VALUES ($1, $2, $2, 'ACCEPTED', $3, $4, $2)`, acknowledgement.GetAcknowledgementId(), commandID, acknowledgement.GetReceivedAt().AsTime(), acknowledgement.GetGatewayId())
	if err != nil {
		t.Fatal(err)
	}
	transitionCommand(t, ctx, pool, commandID, "SENT", "ACKNOWLEDGED", time.Now().UTC())
}

func commandMessage(command commandIntent) *gridosv1.CommandIntent {
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
	databasePath := os.Getenv("GRIDOS_GATEWAY_DATABASE")
	if databasePath == "" {
		databasePath = filepath.Join(repositoryRoot(t), ".local/demo/gateway.db")
	}
	database, err := sql.Open("sqlite", databasePath)
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
