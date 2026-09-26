package tests

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
	"github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/gateway"
	"github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/protocol"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestGatewayProtocol(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 12, 18, 0, 0, 0, time.UTC)
	store, err := gateway.Open(ctx, filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	commandPath, commandHandler := gridosv1connect.NewCommandServiceHandler(protocol.NewCommandHandler(store, "gateway-1", func() time.Time { return now }))
	commandServer := newGRPCServer(t, commandPath, commandHandler)
	commandClient := gridosv1connect.NewCommandServiceClient(commandServer.Client(), commandServer.URL, connect.WithGRPC())
	command := &gridosv1.CommandIntent{
		CommandId:      "command-1",
		IdempotencyKey: "event-1-device-1-v1",
		DeviceId:       "device-1",
		Generation:     1,
		SetpointKw:     3.2,
		EffectiveAt:    timestamppb.New(now),
		ExpiresAt:      timestamppb.New(now.Add(time.Hour)),
	}
	commandResponse, err := commandClient.SubmitCommand(ctx, connect.NewRequest(&gridosv1.SubmitCommandRequest{CommandIntent: command}))
	if err != nil {
		t.Fatal(err)
	}
	if commandResponse.Msg.GetAcknowledgement().GetReceiptStatus() != gridosv1.CommandReceiptStatus_COMMAND_RECEIPT_STATUS_ACCEPTED {
		t.Fatalf("acknowledgement=%+v", commandResponse.Msg.GetAcknowledgement())
	}
	telemetryService := &telemetryRecorder{now: now}
	telemetryPath, telemetryHandler := gridosv1connect.NewTelemetryServiceHandler(telemetryService)
	telemetryServer := newGRPCServer(t, telemetryPath, telemetryHandler)
	publisher := protocol.NewTelemetryPublisher(gridosv1connect.NewTelemetryServiceClient(telemetryServer.Client(), telemetryServer.URL, connect.WithGRPC()), "gateway-1")
	observation := &gridosv1.TelemetryObservation{ObservationId: "observation-1", DeviceId: "device-1"}
	if err := publisher.Publish(ctx, observation); err != nil {
		t.Fatal(err)
	}
	if len(telemetryService.observations) != 1 || telemetryService.observations[0].GetObservationId() != "observation-1" {
		t.Fatalf("observations=%+v", telemetryService.observations)
	}
}

type telemetryRecorder struct {
	now          time.Time
	observations []*gridosv1.TelemetryObservation
}

func (recorder *telemetryRecorder) PublishTelemetry(_ context.Context, request *connect.Request[gridosv1.PublishTelemetryRequest]) (*connect.Response[gridosv1.PublishTelemetryResponse], error) {
	recorder.observations = append(recorder.observations, request.Msg.GetObservations()...)
	ids := make([]string, 0, len(request.Msg.GetObservations()))
	for _, observation := range request.Msg.GetObservations() {
		ids = append(ids, observation.GetObservationId())
	}
	return connect.NewResponse(&gridosv1.PublishTelemetryResponse{DurableReceiptId: "receipt-1", ObservationIds: ids, DurablyReceivedAt: timestamppb.New(recorder.now)}), nil
}

func newGRPCServer(t *testing.T, path string, handler http.Handler) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	server := httptest.NewUnstartedServer(mux)
	server.EnableHTTP2 = true
	server.StartTLS()
	server.Client().Transport.(*http.Transport).TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	t.Cleanup(server.Close)
	return server
}
