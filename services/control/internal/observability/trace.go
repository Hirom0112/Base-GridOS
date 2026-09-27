package observability

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type traceIdentity struct {
	correlationID string
	workflowID    string
	fallback      bool
	derived       bool
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

func WithActivityTraceIDs(ctx context.Context, correlationID, eventID string) context.Context {
	identity := traceIdentity{correlationID: correlationID, workflowID: eventID}
	validEvent := safeIdentifier(eventID)
	validCorrelation := safeIdentifier(correlationID)
	if !validEvent || !validCorrelation {
		digest := sha256.Sum256([]byte(eventID))
		identity.workflowID = "event-" + hex.EncodeToString(digest[:16])
		identity.derived = true
	}
	if !validCorrelation {
		identity.correlationID = identity.workflowID
		identity.fallback = true
	}
	return context.WithValue(ctx, traceIdentityKey{}, identity)
}

func TraceIDs(ctx context.Context) (string, string) {
	identity, ok := ctx.Value(traceIdentityKey{}).(traceIdentity)
	if !ok {
		return "unavailable", "unavailable"
	}
	return identity.correlationID, identity.workflowID
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
	if identity.fallback {
		span.SetAttributes(attribute.Bool("identity_fallback", true))
	}
	if identity.derived {
		span.SetAttributes(attribute.Bool("identity_derived", true))
	}
}

func (traceIdentityProcessor) OnEnd(sdktrace.ReadOnlySpan) {}

func (traceIdentityProcessor) Shutdown(context.Context) error {
	return nil
}

func (traceIdentityProcessor) ForceFlush(context.Context) error {
	return nil
}
