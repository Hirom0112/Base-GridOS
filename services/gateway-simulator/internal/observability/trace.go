package observability

import (
	"context"
	"errors"
	"io"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func NewTraceProvider(writer io.Writer) (*sdktrace.TracerProvider, error) {
	if writer == nil {
		return nil, errors.New("trace writer is required")
	}
	exporter, err := stdouttrace.New(stdouttrace.WithWriter(writer))
	if err != nil {
		return nil, err
	}
	return sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(traceIdentityProcessor{}),
		sdktrace.WithBatcher(&scrubbedExporter{next: exporter}),
	), nil
}

func safeIdentity(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	lower := strings.ToLower(value)
	for _, forbidden := range []string{"site-", "device-", "member-", "credential", "travel", "bearer"} {
		if strings.Contains(lower, forbidden) {
			return false
		}
	}
	return true
}

type traceIdentityProcessor struct{}

func (traceIdentityProcessor) OnStart(_ context.Context, span sdktrace.ReadWriteSpan) {
	span.SetAttributes(attribute.String("correlation_id", "unavailable"), attribute.String("workflow_id", "unavailable"))
}

func (traceIdentityProcessor) OnEnd(sdktrace.ReadOnlySpan) {}

func (traceIdentityProcessor) Shutdown(context.Context) error {
	return nil
}

func (traceIdentityProcessor) ForceFlush(context.Context) error {
	return nil
}

type scrubbedExporter struct {
	next sdktrace.SpanExporter
}

func (s *scrubbedExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	clean := make([]sdktrace.ReadOnlySpan, len(spans))
	for index, span := range spans {
		clean[index] = safeSpan{ReadOnlySpan: span}
	}
	return s.next.ExportSpans(ctx, clean)
}

func (s *scrubbedExporter) Shutdown(ctx context.Context) error {
	return s.next.Shutdown(ctx)
}

type safeSpan struct {
	sdktrace.ReadOnlySpan
}

func (s safeSpan) Name() string {
	return "gridos.gateway"
}

func (s safeSpan) Attributes() []attribute.KeyValue {
	clean := make([]attribute.KeyValue, 0, 2)
	for _, item := range s.ReadOnlySpan.Attributes() {
		if (item.Key == "correlation_id" || item.Key == "workflow_id") && item.Value.Type() == attribute.STRING && safeIdentity(item.Value.AsString()) {
			clean = append(clean, item)
		}
	}
	return clean
}

func (s safeSpan) Events() []sdktrace.Event {
	return nil
}

func (s safeSpan) Links() []sdktrace.Link {
	return nil
}

func (s safeSpan) Status() sdktrace.Status {
	return sdktrace.Status{Code: s.ReadOnlySpan.Status().Code}
}

func (s safeSpan) Resource() *resource.Resource {
	return resource.Empty()
}
