package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
)

type replacementHandler struct {
	gridosv1connect.UnimplementedOptimizationServiceHandler
	request *gridosv1.ReplaceRequest
}

func (handler *replacementHandler) Replace(_ context.Context, request *connect.Request[gridosv1.ReplaceRequest]) (*connect.Response[gridosv1.ReplaceResponse], error) {
	handler.request = request.Msg
	return connect.NewResponse(&gridosv1.ReplaceResponse{ReplacementPlan: &gridosv1.DispatchPlan{EventId: request.Msg.GetCurrent().GetEventId(), PlanVersion: 4}}), nil
}

func TestConnectOptimizerReplacement(t *testing.T) {
	handler := &replacementHandler{}
	path, service := gridosv1connect.NewOptimizationServiceHandler(handler)
	mux := http.NewServeMux()
	mux.Handle(path, service)
	server := httptest.NewServer(mux)
	defer server.Close()
	optimizer := NewConnectOptimizer(gridosv1connect.NewOptimizationServiceClient(http.DefaultClient, server.URL))
	request := &gridosv1.ReplaceRequest{
		Current:          &gridosv1.OptimizationRequest{EventId: "event-replace", PlanVersion: 4},
		ApprovedPlan:     &gridosv1.DispatchPlan{EventId: "event-replace", PlanVersion: 3},
		DroppedDeviceIds: []string{"device-a"}, EnvelopeDeviceIds: []string{"device-a", "device-b"},
		IdempotencyKey: "replace-event-4",
	}
	response, err := optimizer.Replace(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if handler.request.GetIdempotencyKey() != "replace-event-4" || handler.request.GetApprovedPlan().GetPlanVersion() != 3 || len(handler.request.GetDroppedDeviceIds()) != 1 || len(handler.request.GetEnvelopeDeviceIds()) != 2 || response.GetReplacementPlan().GetPlanVersion() != 4 {
		t.Fatalf("replacement request = %#v, response = %#v", handler.request, response)
	}
}
