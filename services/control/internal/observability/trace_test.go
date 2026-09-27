package observability

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/attribute"
)

func TestTraceCarriesSafeCorrelationAndWorkflow(t *testing.T) {
	t.Parallel()
	exporter := &capturedSpans{}
	provider := NewTracerProvider(exporter)
	ctx, err := WithTraceIDs(context.Background(), "correlation-1", "workflow-1")
	if err != nil {
		t.Fatal(err)
	}
	_, span := provider.Tracer("gridos").Start(ctx, "dispatch.plan")
	span.SetAttributes(attribute.String("site_id", "site-private-123"))
	span.End()
	if err := provider.ForceFlush(ctx); err != nil {
		t.Fatal(err)
	}
	if err := provider.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if len(exporter.spans) != 1 {
		t.Fatalf("exported spans: %d", len(exporter.spans))
	}
	attributes := exporter.spans[0].Attributes()
	if len(attributes) != 2 {
		t.Fatalf("safe trace attributes: %+v", attributes)
	}
	values := map[string]string{}
	for _, item := range attributes {
		values[string(item.Key)] = item.Value.AsString()
	}
	if values["correlation_id"] != "correlation-1" || values["workflow_id"] != "workflow-1" {
		t.Fatalf("missing trace identities: %+v", values)
	}
}

func TestTraceRejectsPrivateIdentity(t *testing.T) {
	t.Parallel()
	if _, err := WithTraceIDs(context.Background(), "site-private-123", "workflow-1"); err == nil {
		t.Fatal("private site accepted as trace correlation")
	}
	if _, err := WithTraceIDs(context.Background(), "correlation-1", "workflow-1"); err != nil {
		t.Fatal(err)
	}
}
