package api

import (
	"context"
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
	storagepublisher "github.com/Hirom0112/Base-GridOS/services/control/internal/storage/publisher"
	"google.golang.org/protobuf/types/known/timestamppb"
)

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
	sites, telemetryTwin, err := fleet.Load(filepath.Join(root, "testdata/fleets/austin-5000.jsonl"), twin, now)
	if err != nil {
		t.Fatal(err)
	}
	deviceID := sites[0].GetDevices()[0].GetDeviceId()
	telemetryTwin.Accept(&gridosv1.TelemetryObservation{
		ObservationId: "lifecycle-observation", DeviceId: deviceID, Sequence: 1, ObservationTime: timestamppb.New(now),
		ValueState: gridosv1.ValueState_VALUE_STATE_PRESENT, StateOfEnergyPercent: 74,
		OperatingState: &gridosv1.TelemetryObservation_OnGrid{OnGrid: &gridosv1.OnGrid{ObservedAt: timestamppb.New(now)}},
	})
	client := apiH2Client()
	events := NewPostgresEventStore(pool)
	publisher := storagepublisher.New(storagepublisher.Config{
		Pool: pool, Client: gridosv1connect.NewCommandServiceClient(client, "http://"+gatewayAddress, connect.WithGRPC()),
		AuthorizationToken: "Bearer lifecycle-token", BatchSize: 10, LeaseDuration: time.Second, AcknowledgementTimeout: 2 * time.Second,
		Now: time.Now, Interval: integrationInterval,
	})
	service := NewService(events, twin, sites, time.Now)
	service.SetDispatcher(&Dispatcher{
		Events: events, Snapshots: NewFleetSnapshotter(twin, sites, time.Now),
		Optimizer: NewConnectOptimizer(gridosv1connect.NewOptimizationServiceClient(client, "http://"+decisionAddress, connect.WithGRPC())),
		Safety:    IndependentSafetyGate{}, Approval: NewStoredApprovalGate(events), Commands: NewCommandPipeline(pool, publisher), Now: time.Now,
	})
	control := httptest.NewServer(NewHandler(service))
	defer control.Close()
	dispatch := gridosv1connect.NewDispatchServiceClient(http.DefaultClient, control.URL)
	begin := now.Add(time.Minute)
	create := connect.NewRequest(&gridosv1.CreateEventRequestRequest{EventRequest: &gridosv1.EventRequest{
		RequestId: "lifecycle-event", EventType: "GRID_SERVICE", BeginTime: timestamppb.New(begin), EndTime: timestamppb.New(begin.Add(time.Hour)),
		TargetKw: 1, MeasurementBoundary: gridosv1.MeasurementBoundary_MEASUREMENT_BOUNDARY_METER_NET_EXPORT, LoadZones: []string{"LZ_AEN"}, CorrelationId: "lifecycle",
	}, IdempotencyKey: "create-lifecycle"})
	create.Header().Set(roleHeader, "operator")
	created, err := dispatch.CreateEventRequest(context.Background(), create)
	if err != nil {
		t.Fatal(err)
	}
	if created.Msg.GetEvent().GetState() != gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_VALIDATED || created.Msg.GetEvent().GetPlanVersion() != 1 {
		t.Fatalf("created event = %#v, want validated plan 1", created.Msg.GetEvent())
	}
	approve := connect.NewRequest(&gridosv1.ApproveEventRequest{EventId: "lifecycle-event", PlanVersion: 1, IdempotencyKey: "approve-lifecycle", ApprovedBy: "approver-1", ApprovedAt: timestamppb.Now()})
	approve.Header().Set(roleHeader, "approver")
	if _, err = dispatch.ApproveEvent(context.Background(), approve); err != nil {
		t.Fatal(err)
	}
	launch := connect.NewRequest(&gridosv1.LaunchEventRequest{EventId: "lifecycle-event", PlanVersion: 1, IdempotencyKey: "launch-lifecycle", RequestedBy: "approver-1", RequestedAt: timestamppb.Now()})
	launch.Header().Set(roleHeader, "approver")
	launched, err := dispatch.LaunchEvent(context.Background(), launch)
	if err != nil {
		t.Fatal(err)
	}
	want := map[gridosv1.DispatchEventState]bool{
		gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_SENT:                      true,
		gridosv1.DispatchEventState_DISPATCH_EVENT_STATE_ACKNOWLEDGED_OR_UNCERTAIN: true,
	}
	if !want[launched.Msg.GetEvent().GetState()] {
		t.Fatalf("launched state = %s", launched.Msg.GetEvent().GetState())
	}
	get := connect.NewRequest(&gridosv1.GetEventRequest{EventId: "lifecycle-event"})
	get.Header().Set(roleHeader, "operator")
	loaded, err := dispatch.GetEvent(context.Background(), get)
	if err != nil || !want[loaded.Msg.GetEvent().GetState()] {
		t.Fatalf("loaded event = %#v, %v", loaded, err)
	}
}

func integrationInterval(command storage.ClaimedCommand, now time.Time) storage.FeasiblePowerInterval {
	return storage.FeasiblePowerInterval{DeviceID: command.DeviceID, IntervalBegin: now, IntervalEnd: command.ExpiresAt, LowerKW: min(0, command.SetpointKW), UpperKW: max(0, command.SetpointKW), PossiblyAcceptedCommandID: command.CommandID, PossiblyAcceptedSetpointKW: command.SetpointKW, PossiblyAcceptedEffectiveAt: command.EffectiveAt, PossiblyAcceptedExpiresAt: command.ExpiresAt, FreshTelemetryObservedAt: now, DerivedAt: now, CorrelationID: command.CorrelationID}
}

func apiH2Client() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	protocols := new(http.Protocols)
	protocols.SetUnencryptedHTTP2(true)
	transport.Protocols = protocols
	return &http.Client{Transport: transport, Timeout: 10 * time.Second}
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
	t.Helper()
	command := exec.Command(name, arguments...)
	command.Dir = root
	command.Env = append(os.Environ(), "GRIDOS_GATEWAY_TOKEN=Bearer lifecycle-token")
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
