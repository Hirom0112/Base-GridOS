package dispatch

import (
	"context"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/observability"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"google.golang.org/protobuf/proto"
)

type traceCaptureOptimizer struct {
	activityOptimizer
	identities map[string][2]string
}

func (optimizer *traceCaptureOptimizer) Forecast(ctx context.Context, request *gridosv1.ForecastRequest) (*gridosv1.ForecastResponse, error) {
	correlationID, workflowID := observability.TraceIDs(ctx)
	optimizer.identities["Forecast"] = [2]string{correlationID, workflowID}
	return optimizer.activityOptimizer.Forecast(ctx, request)
}

func (optimizer *traceCaptureOptimizer) Optimize(ctx context.Context, request *gridosv1.OptimizationRequest) (*gridosv1.DispatchPlan, error) {
	correlationID, workflowID := observability.TraceIDs(ctx)
	optimizer.identities["Optimize"] = [2]string{correlationID, workflowID}
	return optimizer.activityOptimizer.Optimize(ctx, request)
}

func (optimizer *traceCaptureOptimizer) Replace(ctx context.Context, request *gridosv1.ReplaceRequest) (*gridosv1.ReplaceResponse, error) {
	correlationID, workflowID := observability.TraceIDs(ctx)
	optimizer.identities["Replace"] = [2]string{correlationID, workflowID}
	return optimizer.activityOptimizer.Replace(ctx, request)
}

func TestPlanningActivitiesSeedDecisionTraceContext(t *testing.T) {
	harness := newActivityHarness(t)
	base := harness.activities.Dispatcher.Optimizer.(activityOptimizer)
	optimizer := &traceCaptureOptimizer{activityOptimizer: base, identities: make(map[string][2]string)}
	harness.activities.Dispatcher.Optimizer = optimizer
	frozen := harness.freeze(t)
	_, err := harness.activities.RequestPlan(context.Background(), frozen)
	require.NoError(t, err)
	want := [2]string{"correlation-1", "event-1"}
	require.Equal(t, want, optimizer.identities["Forecast"])
	require.Equal(t, want, optimizer.identities["Optimize"])
}

func TestPlanningActivityMissingCorrelationOnTravelEvent(t *testing.T) {
	harness := newActivityHarness(t)
	request := proto.Clone(harness.input.Request).(*gridosv1.EventRequest)
	request.RequestId = "travel-flex-x"
	request.CorrelationId = ""
	request.LoadZones = []string{}
	_, err := harness.events.Create(context.Background(), request, "create-travel-flex-x", time.Now())
	require.NoError(t, err)
	exporter := tracetest.NewInMemoryExporter()
	provider := observability.NewTracerProvider(exporter)
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
		otel.SetTracerProvider(previous)
	})
	base := harness.activities.Dispatcher.Optimizer.(activityOptimizer)
	optimizer := &traceCaptureOptimizer{activityOptimizer: base, identities: make(map[string][2]string)}
	harness.activities.Dispatcher.Optimizer = optimizer
	ctx, span := harness.activities.startActivity(context.Background(), request.GetRequestId(), "FreezeInputs")
	_, err = harness.activities.forecast(ctx, &gridosv1.OptimizationRequest{EventId: request.GetRequestId()}, request.GetRequestId(), time.Second)
	span.End()
	require.NoError(t, err)
	correlationID, workflowID := observability.TraceIDs(ctx)
	require.Regexp(t, `^event-[0-9a-f]{32}$`, correlationID)
	require.Equal(t, correlationID, workflowID)
	require.Equal(t, [2]string{correlationID, workflowID}, optimizer.identities["Forecast"])
	require.NoError(t, provider.ForceFlush(context.Background()))
	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	derived := false
	for _, attribute := range spans[0].Attributes {
		if string(attribute.Key) == "identity_derived" {
			derived = attribute.Value.AsBool()
		}
	}
	require.True(t, derived)
}

func TestFreezeInputsActivitySpanUsesDurableCorrelation(t *testing.T) {
	harness := newActivityHarness(t)
	exporter := tracetest.NewInMemoryExporter()
	provider := observability.NewTracerProvider(exporter)
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
		otel.SetTracerProvider(previous)
	})
	harness.freeze(t)
	require.NoError(t, provider.ForceFlush(context.Background()))
	spans := exporter.GetSpans()
	require.NotEmpty(t, spans)
	values := map[string]string{}
	for _, item := range spans[0].Attributes {
		values[string(item.Key)] = item.Value.AsString()
	}
	require.Equal(t, "correlation-1", values["correlation_id"])
	require.Equal(t, "event-1", values["workflow_id"])
}

func TestIssueReplacementActivitySpanUsesDurableCorrelation(t *testing.T) {
	harness := newActivityHarness(t)
	harness.persist(t)
	exporter := tracetest.NewInMemoryExporter()
	provider := observability.NewTracerProvider(exporter)
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
		otel.SetTracerProvider(previous)
	})
	require.Error(t, harness.activities.IssueReplacement(context.Background(), ReplacementCommand{
		EventID: harness.input.EventID, Request: harness.input.Request,
		DroppedDeviceIDs: []string{"device-1"}, EnvelopeDeviceIDs: []string{"device-1", "device-2"}, Generation: 2,
	}))
	require.NoError(t, provider.ForceFlush(context.Background()))
	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	values := map[string]string{}
	for _, item := range spans[0].Attributes {
		values[string(item.Key)] = item.Value.AsString()
	}
	require.Equal(t, "correlation-1", values["correlation_id"])
	require.Equal(t, "event-1", values["workflow_id"])
}
