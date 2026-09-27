package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/observability"
)

type traceOptimizationHandler struct {
	gridosv1connect.UnimplementedOptimizationServiceHandler
	headers map[string]http.Header
}

func (handler *traceOptimizationHandler) Forecast(_ context.Context, request *connect.Request[gridosv1.ForecastRequest]) (*connect.Response[gridosv1.ForecastResponse], error) {
	handler.headers["Forecast"] = request.Header().Clone()
	return connect.NewResponse(&gridosv1.ForecastResponse{}), nil
}

func (handler *traceOptimizationHandler) Optimize(_ context.Context, request *connect.Request[gridosv1.OptimizeRequest]) (*connect.Response[gridosv1.OptimizeResponse], error) {
	handler.headers["Optimize"] = request.Header().Clone()
	return connect.NewResponse(&gridosv1.OptimizeResponse{Plan: &gridosv1.DispatchPlan{}}), nil
}

func (handler *traceOptimizationHandler) Replace(_ context.Context, request *connect.Request[gridosv1.ReplaceRequest]) (*connect.Response[gridosv1.ReplaceResponse], error) {
	handler.headers["Replace"] = request.Header().Clone()
	return connect.NewResponse(&gridosv1.ReplaceResponse{ReplacementPlan: &gridosv1.DispatchPlan{}}), nil
}

func TestConnectOptimizerForwardsTraceIdentityOnEveryRPC(t *testing.T) {
	handler := &traceOptimizationHandler{headers: make(map[string]http.Header)}
	path, service := gridosv1connect.NewOptimizationServiceHandler(handler)
	mux := http.NewServeMux()
	mux.Handle(path, service)
	server := httptest.NewServer(mux)
	defer server.Close()
	optimizer := NewConnectOptimizer(gridosv1connect.NewOptimizationServiceClient(http.DefaultClient, server.URL))
	ctx, err := observability.WithTraceIDs(context.Background(), "correlation-7", "workflow-7")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := optimizer.Forecast(ctx, &gridosv1.ForecastRequest{}); err != nil {
		t.Fatal(err)
	}
	if _, err := optimizer.Optimize(ctx, &gridosv1.OptimizationRequest{}); err != nil {
		t.Fatal(err)
	}
	if _, err := optimizer.Replace(ctx, &gridosv1.ReplaceRequest{}); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"Forecast", "Optimize", "Replace"} {
		headers := handler.headers[method]
		if headers.Get("X-Correlation-Id") != "correlation-7" || headers.Get("X-Workflow-Id") != "workflow-7" {
			t.Fatalf("%s trace headers = %v", method, headers)
		}
	}
}
