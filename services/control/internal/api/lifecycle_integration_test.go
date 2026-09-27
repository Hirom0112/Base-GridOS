package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/storage"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestTemporalLifecycleRequiresWorker(t *testing.T) {
	pool := apiTestDatabase(t)
	temporalClient, err := client.Dial(client.Options{HostPort: "127.0.0.1:7233"})
	if err != nil {
		t.Fatal(err)
	}
	defer temporalClient.Close()
	events := NewPostgresEventStore(pool)
	service := NewService(events, fleet.NewTwin(time.Minute), nil, time.Now)
	taskQueue := "api-without-worker-" + fmt.Sprint(time.Now().UnixNano())
	service.SetWorkflowClient(temporalClient, taskQueue)
	control := httptest.NewServer(NewHandler(service))
	defer control.Close()
	dispatchClient := gridosv1connect.NewDispatchServiceClient(http.DefaultClient, control.URL)
	now := time.Now().UTC()
	request := connect.NewRequest(&gridosv1.CreateEventRequestRequest{EventRequest: &gridosv1.EventRequest{
		RequestId: taskQueue, EventType: "GRID_SERVICE", BeginTime: timestamppb.New(now.Add(time.Minute)), EndTime: timestamppb.New(now.Add(time.Hour)),
		TargetKw: 1, MeasurementBoundary: gridosv1.MeasurementBoundary_MEASUREMENT_BOUNDARY_METER_NET_EXPORT, LoadZones: []string{"LZ_AEN"}, CorrelationId: taskQueue,
	}, IdempotencyKey: taskQueue})
	request.Header().Set(roleHeader, "operator")
	created, err := dispatchClient.CreateEventRequest(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if created.Msg.GetEvent().GetState() != gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_REQUESTED {
		t.Fatalf("event state = %s, want REQUESTED", created.Msg.GetEvent().GetState())
	}
	description, err := temporalClient.DescribeWorkflowExecution(context.Background(), taskQueue, "")
	if err != nil {
		t.Fatal(err)
	}
	if description.WorkflowExecutionInfo.Status != enums.WORKFLOW_EXECUTION_STATUS_RUNNING {
		t.Fatalf("workflow status = %s, want RUNNING", description.WorkflowExecutionInfo.Status)
	}
	_ = temporalClient.TerminateWorkflow(context.Background(), taskQueue, "", "test cleanup")
}

func TestControlLifecycleWithRealDecisionAndGateway(t *testing.T) {
	root := apiRepositoryRoot(t)
	now := time.Now().UTC()
	decisionAddress := availableAPIAddress(t)
	gatewayAddress := availableAPIAddress(t)
	startAPIProcess(t, root, "uv", "run", "--project", "services/decision", "python", "-m", "gridos.server", "--port", port(decisionAddress))
	gatewayDatabase := filepath.Join(t.TempDir(), "gateway.db")
	startAPIProcess(t, root, "go", "run", "./services/gateway-simulator/cmd/gateway-simulator", "--address", gatewayAddress, "--database", gatewayDatabase, "--fleet", "testdata/fleets/austin-5000.jsonl", "--gateway-id", "gateway-integration", "--scenario-start", now.Format(time.RFC3339))
	waitForAPI(t, decisionAddress)
	waitForAPI(t, gatewayAddress)

	pool := apiTestDatabase(t)
	twin := fleet.NewTwin(time.Minute)
	fleetPath := filepath.Join(root, "testdata/fleets/texas-50.jsonl")
	sites, telemetryTwin, err := fleet.Load(fleetPath, twin, now)
	if err != nil {
		t.Fatal(err)
	}
	deviceID := sites[0].GetDevices()[0].GetDeviceId()
	telemetryTwin.Accept(&gridosv1.TelemetryObservation{
		ObservationId: "lifecycle-observation", DeviceId: deviceID, Sequence: 1, ObservationTime: timestamppb.New(now),
		ValueState: gridosv1.ValueState_VALUE_STATE_PRESENT, StateOfEnergyPercent: 74,
		OperatingState: &gridosv1.TelemetryObservation_OnGrid{OnGrid: &gridosv1.OnGrid{ObservedAt: timestamppb.New(now)}},
	})
	if _, err = storage.NewTelemetryStore(pool).Write(context.Background(), "lifecycle-gateway", []*gridosv1.TelemetryObservation{{
		ObservationId: "worker-lifecycle-observation", DeviceId: deviceID, Sequence: 1, ObservationTime: timestamppb.New(now),
		ValueState: gridosv1.ValueState_VALUE_STATE_PRESENT, StateOfEnergyPercent: 74,
		OperatingState: &gridosv1.TelemetryObservation_OnGrid{OnGrid: &gridosv1.OnGrid{ObservedAt: timestamppb.New(now)}},
	}}); err != nil {
		t.Fatal(err)
	}
	events := NewPostgresEventStore(pool)
	temporalClient, err := client.Dial(client.Options{HostPort: "127.0.0.1:7233"})
	if err != nil {
		t.Fatal(err)
	}
	defer temporalClient.Close()
	taskQueue := "lifecycle-worker-" + fmt.Sprint(time.Now().UnixNano())
	eventID := "lifecycle-event-" + fmt.Sprint(time.Now().UnixNano())
	service := NewService(events, twin, sites, time.Now)
	service.SetWorkflowClient(temporalClient, taskQueue)
	database := pool.Config().ConnConfig
	startAPIProcessWithEnv(t, root, []string{
		fmt.Sprintf("GRIDOS_DATABASE_URL=postgres://%s:%s@%s:%d/%s?sslmode=disable", database.User, database.Password, database.Host, database.Port, database.Database),
		"GRIDOS_DECISION_ADDR=http://" + decisionAddress,
		"GRIDOS_GATEWAY_ADDR=http://" + gatewayAddress,
		"GRIDOS_FLEET=" + fleetPath,
		"GRIDOS_TASK_QUEUE=" + taskQueue,
		"GRIDOS_CODE_VERSION=api-lifecycle-test",
	}, "go", "run", "./services/control/cmd/worker")
	control := httptest.NewServer(NewHandler(service))
	defer control.Close()
	dispatch := gridosv1connect.NewDispatchServiceClient(http.DefaultClient, control.URL)
	begin := now.Add(time.Minute)
	create := connect.NewRequest(&gridosv1.CreateEventRequestRequest{EventRequest: &gridosv1.EventRequest{
		RequestId: eventID, EventType: "GRID_SERVICE", BeginTime: timestamppb.New(begin), EndTime: timestamppb.New(begin.Add(time.Hour)),
		TargetKw: 1, MeasurementBoundary: gridosv1.MeasurementBoundary_MEASUREMENT_BOUNDARY_METER_NET_EXPORT, LoadZones: []string{sites[0].GetSite().GetLoadZone()}, CorrelationId: "lifecycle",
	}, IdempotencyKey: "create-lifecycle"})
	create.Header().Set(roleHeader, "operator")
	created, err := dispatch.CreateEventRequest(context.Background(), create)
	if err != nil {
		t.Fatal(err)
	}
	if created.Msg.GetEvent().GetState() != gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_REQUESTED {
		t.Fatalf("created event = %#v, want requested", created.Msg.GetEvent())
	}
	if _, err = temporalClient.DescribeWorkflowExecution(context.Background(), eventID, ""); err != nil {
		t.Fatal(err)
	}
	waitForEventState(t, dispatch, eventID, gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_VALIDATED)
	approve := connect.NewRequest(&gridosv1.ApproveEventRequest{EventId: eventID, PlanVersion: 1, IdempotencyKey: "approve-lifecycle", ApprovedBy: "approver-1", ApprovedAt: timestamppb.Now()})
	approve.Header().Set(roleHeader, "approver")
	if _, err = dispatch.ApproveEvent(context.Background(), approve); err != nil {
		t.Fatal(err)
	}
	launch := connect.NewRequest(&gridosv1.LaunchEventRequest{EventId: eventID, PlanVersion: 1, IdempotencyKey: "launch-lifecycle", RequestedBy: "approver-1", RequestedAt: timestamppb.Now()})
	launch.Header().Set(roleHeader, "approver")
	if _, err = dispatch.LaunchEvent(context.Background(), launch); err != nil {
		t.Fatal(err)
	}
	waitForEventState(t, dispatch, eventID, gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_SENT)
}

func waitForEventState(t *testing.T, dispatch gridosv1connect.DispatchServiceClient, eventID string, minimum gridosv1.DispatchEventState) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		request := connect.NewRequest(&gridosv1.GetEventRequest{EventId: eventID})
		request.Header().Set(roleHeader, "operator")
		response, err := dispatch.GetEvent(context.Background(), request)
		if err == nil && response.Msg.GetEvent().GetState() >= minimum {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("event %s did not reach %s", eventID, minimum)
}

func apiRepositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate repository")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../../.."))
}

func availableAPIAddress(t *testing.T) string {
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

func port(address string) string {
	_, value, _ := net.SplitHostPort(address)
	return value
}

func startAPIProcess(t *testing.T, root, name string, arguments ...string) {
	startAPIProcessWithEnv(t, root, nil, name, arguments...)
}

func startAPIProcessWithEnv(t *testing.T, root string, environment []string, name string, arguments ...string) {
	t.Helper()
	command := exec.Command(name, arguments...)
	command.Dir = root
	command.Env = append(os.Environ(), append(environment, "GRIDOS_GATEWAY_TOKEN=Bearer lifecycle-token")...)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = command.Process.Kill()
		_, _ = command.Process.Wait()
	})
}

func waitForAPI(t *testing.T, address string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		connection, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
		if err == nil {
			_ = connection.Close()
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("process did not listen on %s", address)
}
