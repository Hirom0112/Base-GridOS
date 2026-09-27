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
	commandPath, commandHandler := gridosv1connect.NewCommandServiceHandler(protocol.NewCommandHandler(store, "gateway-1", "Bearer test-token", func() time.Time { return now }))
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
	commandRequest := connect.NewRequest(&gridosv1.SubmitCommandRequest{CommandIntent: command})
	commandRequest.Header().Set("Authorization", "Bearer test-token")
	commandResponse, err := commandClient.SubmitCommand(ctx, commandRequest)
	if err != nil {
		t.Fatal(err)
	}
	if commandResponse.Msg.GetAcknowledgement().GetReceiptStatus() != gridosv1.CommandReceiptStatus_COMMAND_RECEIPT_STATUS_ACCEPTED {
		t.Fatalf("acknowledgement=%+v", commandResponse.Msg.GetAcknowledgement())
	}
}

func newGRPCServer(t *testing.T, path string, handler http.Handler) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	server := httptest.NewUnstartedServer(mux)
	server.EnableHTTP2 = true
	server.StartTLS()
	server.Client().Transport.(*http.Transport).TLSClientConfig.MinVersion = tls.VersionTLS12
	t.Cleanup(server.Close)
	return server
}
