package observability

import (
	"context"
	"log/slog"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type ScrubbedLogHandler struct {
	next slog.Handler
}

func NewScrubbedLogHandler(next slog.Handler) *ScrubbedLogHandler {
	return &ScrubbedLogHandler{next: next}
}

func (h *ScrubbedLogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *ScrubbedLogHandler) Handle(ctx context.Context, record slog.Record) error {
	clean := slog.NewRecord(record.Time, record.Level, "gridos.event", record.PC)
	record.Attrs(func(attr slog.Attr) bool {
		if safeLogAttr(attr) {
			clean.AddAttrs(attr)
		}
		return true
	})
	return h.next.Handle(ctx, clean)
}

func (h *ScrubbedLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clean := make([]slog.Attr, 0, len(attrs))
	for _, attr := range attrs {
		if safeLogAttr(attr) {
			clean = append(clean, attr)
		}
	}
	return &ScrubbedLogHandler{next: h.next.WithAttrs(clean)}
}

func (h *ScrubbedLogHandler) WithGroup(string) slog.Handler {
	return &ScrubbedLogHandler{next: h.next.WithGroup("gridos")}
}

func safeLogAttr(attr slog.Attr) bool {
	switch attr.Key {
	case "workflow_id", "correlation_id", "event_id":
		return attr.Value.Kind() == slog.KindString && safeIdentifier(attr.Value.String())
	case "count", "duration_ms", "power_kw", "energy_kwh":
		return attr.Value.Kind() == slog.KindInt64 || attr.Value.Kind() == slog.KindFloat64
	default:
		return false
	}
}

func safeIdentifier(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	lower := strings.ToLower(value)
	for _, forbidden := range []string{"site-", "site_", "device-", "device_", "member-", "member_", "household", "credential", "travel", "bearer"} {
		if strings.Contains(lower, forbidden) {
			return false
		}
	}
	for _, char := range value {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '-' || char == '_' {
			continue
		}
		return false
	}
	return true
}

type ScrubbedSpanExporter struct {
	next sdktrace.SpanExporter
}

func NewScrubbedSpanExporter(next sdktrace.SpanExporter) *ScrubbedSpanExporter {
	return &ScrubbedSpanExporter{next: next}
}

func (e *ScrubbedSpanExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	clean := make([]sdktrace.ReadOnlySpan, len(spans))
	for index, span := range spans {
		clean[index] = scrubbedSpan{ReadOnlySpan: span}
	}
	return e.next.ExportSpans(ctx, clean)
}

func (e *ScrubbedSpanExporter) Shutdown(ctx context.Context) error {
	return e.next.Shutdown(ctx)
}

type scrubbedSpan struct {
	sdktrace.ReadOnlySpan
}

func (s scrubbedSpan) Name() string {
	return "gridos.operation"
}

func (s scrubbedSpan) Attributes() []attribute.KeyValue {
	clean := make([]attribute.KeyValue, 0)
	for _, item := range s.ReadOnlySpan.Attributes() {
		if safeTraceAttr(item) {
			clean = append(clean, item)
		}
	}
	return clean
}

func (s scrubbedSpan) Events() []sdktrace.Event {
	return nil
}

func (s scrubbedSpan) Links() []sdktrace.Link {
	return nil
}

func (s scrubbedSpan) Status() sdktrace.Status {
	return sdktrace.Status{Code: s.ReadOnlySpan.Status().Code}
}

func (s scrubbedSpan) Resource() *resource.Resource {
	return resource.Empty()
}

func (s scrubbedSpan) InstrumentationScope() instrumentation.Scope {
	return instrumentation.Scope{Name: "gridos"}
}

func safeTraceAttr(item attribute.KeyValue) bool {
	switch string(item.Key) {
	case "workflow_id", "correlation_id", "event_id":
		return item.Value.Type() == attribute.STRING && safeIdentifier(item.Value.AsString())
	case "count", "duration_ms", "power_kw", "energy_kwh":
		return item.Value.Type() == attribute.INT64 || item.Value.Type() == attribute.FLOAT64
	case "identity_fallback", "identity_derived":
		return item.Value.Type() == attribute.BOOL
	default:
		return false
	}
}
