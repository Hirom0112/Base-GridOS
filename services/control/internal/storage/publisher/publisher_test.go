package publisher

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/observability"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/storage"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestPublisherAgainstGatewaySimulator(t *testing.T) {
	pool := publisherDatabase(t)
	now := time.Now().UTC()
	command := publisherCommand("gateway-command", now)
	seedPublisherCommand(t, pool, command)
	address := availableAddress(t)
	process := startGateway(t, address, now)
	defer stopProcess(t, process)
	waitForGateway(t, address)
	client := gridosv1connect.NewCommandServiceClient(http.DefaultClient, "http://"+address, connect.WithGRPC())
	publisher := New(Config{
		Pool:                   pool,
		Client:                 client,
		AuthorizationToken:     "Bearer publisher-test",
		BatchSize:              10,
		LeaseDuration:          time.Second,
		AcknowledgementTimeout: 2 * time.Second,
		Now:                    func() time.Time { return now },
		Interval:               intervalFor,
	})
	if err := publisher.PublishBatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertPublisherState(t, pool, command.CommandID, "ACKNOWLEDGED", "PUBLISHED")
}

func TestPublisherBatchEmitsTraceIdentity(t *testing.T) {
	pool := publisherDatabase(t)
	exporter := tracetest.NewInMemoryExporter()
	provider := observability.NewTracerProvider(exporter)
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
		otel.SetTracerProvider(previous)
	})
	ctx, err := observability.WithTraceIDs(context.Background(), "correlation-publisher", "workflow-publisher")
	if err != nil {
		t.Fatal(err)
	}
	publisher := New(Config{Pool: pool, BatchSize: 1, LeaseDuration: time.Second, Now: time.Now})
	if err := publisher.PublishBatch(ctx); err != nil {
		t.Fatal(err)
	}
	if err := provider.ForceFlush(ctx); err != nil {
		t.Fatal(err)
	}
	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("publisher spans = %d, want 1", len(spans))
	}
	values := map[string]string{}
	for _, item := range spans[0].Attributes {
		values[string(item.Key)] = item.Value.AsString()
	}
	if values["correlation_id"] != "correlation-publisher" || values["workflow_id"] != "workflow-publisher" {
		t.Fatalf("publisher trace identity = %v", values)
	}
}

func TestPublisherRetriesSameCommandID(t *testing.T) {
	pool := publisherDatabase(t)
	now := time.Now().UTC()
	command := publisherCommand("retry-command", now)
	seedPublisherCommand(t, pool, command)
	service := &recordingCommandService{failFirst: true, now: now}
	server := commandServer(t, service)
	publisher := New(Config{
		Pool:                   pool,
		Client:                 gridosv1connect.NewCommandServiceClient(server.Client(), server.URL, connect.WithGRPC()),
		AuthorizationToken:     "Bearer publisher-test",
		BatchSize:              1,
		LeaseDuration:          time.Second,
		AcknowledgementTimeout: time.Second,
		Now:                    func() time.Time { return now },
		Interval:               intervalFor,
	})
	before := acknowledgementSamples(t)
	if err := publisher.PublishBatch(context.Background()); err == nil {
		t.Fatal("first delivery succeeded")
	}
	if got := acknowledgementSamples(t); got != before {
		t.Fatalf("failed delivery samples = %v, want %v", got, before)
	}
	now = now.Add(2 * time.Second)
	if err := publisher.PublishBatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := acknowledgementSamples(t); got != before+1 {
		t.Fatalf("accepted delivery samples = %v, want %v", got, before+1)
	}
	if len(service.commandIDs) != 2 || service.commandIDs[0] != service.commandIDs[1] {
		t.Fatalf("retry command IDs = %v", service.commandIDs)
	}
	assertPublisherState(t, pool, command.CommandID, "ACKNOWLEDGED", "PUBLISHED")
}

func acknowledgementSamples(t *testing.T) float64 {
	t.Helper()
	response := httptest.NewRecorder()
	observability.ProcessMetrics.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/metrics", nil))
	for line := range strings.SplitSeq(response.Body.String(), "\n") {
		if value, ok := strings.CutPrefix(line, "gridos_ack_latency_seconds_count "); ok {
			count, err := strconv.ParseFloat(value, 64)
			if err != nil {
				t.Fatal(err)
			}
			return count
		}
	}
	t.Fatal("ack latency metric absent")
	return 0
}

func TestPublisherMarksDeadlineUncertain(t *testing.T) {
	pool := publisherDatabase(t)
	now := time.Now().UTC()
	command := publisherCommand("deadline-command", now)
	following := publisherCommand("following-command", now)
	seedPublisherCommand(t, pool, command)
	if err := storage.InsertCommand(context.Background(), pool, following); err != nil {
		t.Fatal(err)
	}
	service := &recordingCommandService{deadlineCommandID: command.CommandID, now: now}
	server := commandServer(t, service)
	publisher := New(Config{
		Pool:                   pool,
		Client:                 gridosv1connect.NewCommandServiceClient(server.Client(), server.URL, connect.WithGRPC()),
		AuthorizationToken:     "Bearer publisher-test",
		BatchSize:              2,
		LeaseDuration:          time.Second,
		AcknowledgementTimeout: 10 * time.Millisecond,
		Now:                    func() time.Time { return now },
		Interval:               intervalFor,
	})
	if err := publisher.PublishBatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertPublisherState(t, pool, command.CommandID, "UNCERTAIN", "PUBLISHING")
	assertPublisherState(t, pool, following.CommandID, "PERSISTED", "PENDING")
	assertPublisherState(t, pool, following.CommandID, "ACKNOWLEDGED", "PUBLISHED")
}

type recordingCommandService struct {
	mu                sync.Mutex
	commandIDs        []string
	failFirst         bool
	deadlineCommandID string
	now               time.Time
}

func (service *recordingCommandService) SubmitCommand(ctx context.Context, request *connect.Request[gridosv1.SubmitCommandRequest]) (*connect.Response[gridosv1.SubmitCommandResponse], error) {
	service.mu.Lock()
	service.commandIDs = append(service.commandIDs, request.Msg.GetCommandIntent().GetCommandId())
	attempt := len(service.commandIDs)
	service.mu.Unlock()
	if request.Msg.GetCommandIntent().GetCommandId() == service.deadlineCommandID {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if service.failFirst && attempt == 1 {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("unavailable"))
	}
	intent := request.Msg.GetCommandIntent()
	return connect.NewResponse(&gridosv1.SubmitCommandResponse{Acknowledgement: &gridosv1.CommandAcknowledgement{
		AcknowledgementId: intent.GetCommandId() + "-ack",
		CommandId:         intent.GetCommandId(),
		IdempotencyKey:    intent.GetIdempotencyKey(),
		ReceiptStatus:     gridosv1.CommandReceiptStatus_COMMAND_RECEIPT_STATUS_ACCEPTED,
		ReceivedAt:        timestamppb.New(service.now),
		GatewayId:         "gateway-test",
	}}), nil
}

func commandServer(t *testing.T, service *recordingCommandService) *httptest.Server {
	t.Helper()
	path, handler := gridosv1connect.NewCommandServiceHandler(service)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func publisherDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	adminURL := os.Getenv("GRIDOS_DATABASE_URL")
	if adminURL == "" {
		adminURL = "postgres://gridos:gridos@localhost:5432/gridos?sslmode=disable"
	}
	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("gridos_publisher_%d", time.Now().UnixNano())
	identifier := pgx.Identifier{name}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+identifier); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(ctx, "DROP DATABASE "+identifier+" WITH (FORCE)")
		_ = admin.Close(ctx)
	})
	config, err := pgxpool.ParseConfig(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	root := repositoryRoot(t)
	files, err := filepath.Glob(filepath.Join(root, "database/migrations/*.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	for _, path := range files {
		contents, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if _, execErr := pool.Exec(ctx, string(contents)); execErr != nil {
			t.Fatal(execErr)
		}
	}
	return pool
}

func seedPublisherCommand(t *testing.T, pool *pgxpool.Pool, command storage.CommandIntent) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx, `INSERT INTO dispatch_requests
        (request_id, event_type, begin_time, end_time, target_kw, measurement_boundary, load_zones, correlation_id)
        VALUES ('publisher-request', 'GRID_SERVICE', now(), now() + interval '1 hour', 100, 'METER_NET_EXPORT', ARRAY['LZ_AEN'], 'publisher')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO dispatch_events
        (event_id, request_id, state, correlation_id) VALUES ('publisher-event', 'publisher-request', 'APPROVED', 'publisher');
        INSERT INTO input_snapshots (snapshot_id, event_id, captured_at, inputs, provenance, correlation_id)
        VALUES ('publisher-input', 'publisher-event', now(), '{}', '{}', 'publisher');
        INSERT INTO eligibility_snapshots (snapshot_id, event_id, captured_at, eligible_device_ids, exclusions, policy_version, correlation_id)
        VALUES ('publisher-eligibility', 'publisher-event', now(), ARRAY['device-1'], '{}', 'policy-1', 'publisher');
        INSERT INTO plan_versions (event_id, version, input_snapshot_id, eligibility_snapshot_id, plan, solver_version, model_version, correlation_id)
        VALUES ('publisher-event', 1, 'publisher-input', 'publisher-eligibility', '{}', 'solver-1', 'model-1', 'publisher')`)
	if err != nil {
		t.Fatal(err)
	}
	if err = storage.InsertCommand(ctx, pool, command); err != nil {
		t.Fatal(err)
	}
}

func publisherCommand(id string, now time.Time) storage.CommandIntent {
	return storage.CommandIntent{
		CommandID: id, IdempotencyKey: "idempotency-" + id, DeviceID: "device-1",
		EventID: "publisher-event", PlanVersion: 1, Generation: 1, SetpointKW: 3.5,
		IssuedAt: now, EffectiveAt: now.Add(time.Minute), ExpiresAt: now.Add(time.Hour),
		PolicyVersion: "policy-1", CorrelationID: "correlation-" + id,
	}
}

func intervalFor(command storage.ClaimedCommand, now time.Time) storage.FeasiblePowerInterval {
	return storage.FeasiblePowerInterval{
		DeviceID: command.DeviceID, IntervalBegin: now, IntervalEnd: now.Add(5 * time.Minute),
		LowerKW: -3.5, UpperKW: 3.5, LastConfirmedCommandID: "none",
		PossiblyAcceptedCommandID: command.CommandID, PossiblyAcceptedSetpointKW: command.SetpointKW,
		PossiblyAcceptedEffectiveAt: command.EffectiveAt, PossiblyAcceptedExpiresAt: command.ExpiresAt,
		MaxRampKWPerSecond: 1, FreshTelemetryObservedAt: now, DerivedAt: now,
		CorrelationID: command.CorrelationID,
	}
}

func assertPublisherState(t *testing.T, pool *pgxpool.Pool, commandID, commandState, outboxState string) {
	t.Helper()
	var actualCommand, actualOutbox string
	err := pool.QueryRow(context.Background(), `SELECT states.state, outbox.state
        FROM command_outbox AS outbox
        JOIN LATERAL (SELECT state FROM command_states WHERE command_id = outbox.command_id ORDER BY recorded_at DESC LIMIT 1) AS states ON true
        WHERE outbox.command_id = $1`, commandID).Scan(&actualCommand, &actualOutbox)
	if err != nil {
		t.Fatal(err)
	}
	if actualCommand != commandState || actualOutbox != outboxState {
		t.Fatalf("states = %s/%s, want %s/%s", actualCommand, actualOutbox, commandState, outboxState)
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate repository")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../../../../"))
}

func availableAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err = listener.Close(); err != nil {
		t.Fatal(err)
	}
	return address
}

func startGateway(t *testing.T, address string, now time.Time) *exec.Cmd {
	t.Helper()
	root := repositoryRoot(t)
	binary := filepath.Join(t.TempDir(), "gateway-simulator")
	build := exec.Command("go", "build", "-o", binary, "./services/gateway-simulator/cmd/gateway-simulator")
	build.Dir = root
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build gateway: %v: %s", err, output)
	}
	command := exec.Command(binary,
		"-address", address,
		"-database", filepath.Join(t.TempDir(), "gateway.db"),
		"-fleet", filepath.Join(root, "testdata/fleets/texas-50.jsonl"),
		"-gateway-id", "gateway-test",
		"-scenario-start", now.Format(time.RFC3339Nano))
	command.Dir = root
	command.Env = append(os.Environ(), "GRIDOS_GATEWAY_TOKEN=Bearer publisher-test")
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	return command
}

func waitForGateway(t *testing.T, address string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		connection, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
		if err == nil {
			_ = connection.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("gateway did not start")
}

func stopProcess(t *testing.T, command *exec.Cmd) {
	t.Helper()
	if command.Process == nil {
		return
	}
	_ = command.Process.Signal(os.Interrupt)
	if err := command.Wait(); err != nil {
		var exitError *exec.ExitError
		if !errors.As(err, &exitError) || !exitError.Success() {
			t.Log(err)
		}
	}
}
