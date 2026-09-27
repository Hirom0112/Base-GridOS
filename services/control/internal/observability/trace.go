package observability

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type traceIdentity struct {
	correlationID string
	workflowID    string
}

type traceIdentityKey struct{}

func WithTraceIDs(ctx context.Context, correlationID, workflowID string) (context.Context, error) {
	if !safeIdentifier(correlationID) || !safeIdentifier(workflowID) {
		return nil, errors.New("invalid trace identity")
	}
	return context.WithValue(ctx, traceIdentityKey{}, traceIdentity{
		correlationID: correlationID, workflowID: workflowID,
	}), nil
}

func NewTracerProvider(exporter sdktrace.SpanExporter) *sdktrace.TracerProvider {
	return sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(traceIdentityProcessor{}),
		sdktrace.WithBatcher(NewScrubbedSpanExporter(exporter)),
	)
}

type traceIdentityProcessor struct{}

func (traceIdentityProcessor) OnStart(ctx context.Context, span sdktrace.ReadWriteSpan) {
	identity, ok := ctx.Value(traceIdentityKey{}).(traceIdentity)
	if !ok {
		identity = traceIdentity{correlationID: "unavailable", workflowID: "unavailable"}
	}
	span.SetAttributes(
		attribute.String("correlation_id", identity.correlationID),
		attribute.String("workflow_id", identity.workflowID),
	)
}

func (traceIdentityProcessor) OnEnd(sdktrace.ReadOnlySpan) {}

func (traceIdentityProcessor) Shutdown(context.Context) error {
	return nil
}

func (traceIdentityProcessor) ForceFlush(context.Context) error {
	return nil
}
