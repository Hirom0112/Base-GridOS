package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/Hirom0112/Base-GridOS/services/control/internal/analytics"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const privateSite = "site-private-123"
const privateCredential = "credential-private-456"
const privateTravelWindow = "travel-window-private-789"

func TestScrubExporters(t *testing.T) {
	t.Parallel()
	assertScrubbedLog(t)
	assertScrubbedTrace(t)
	assertScrubbedAnalytics(t)
}

func assertScrubbedLog(t *testing.T) {
	t.Helper()
	var output bytes.Buffer
	logger := slog.New(NewScrubbedLogHandler(slog.NewJSONHandler(&output, nil)))
	logger.Info(privateSite,
		slog.String("site_id", privateSite),
		slog.String("command_credential", privateCredential),
		slog.String("travel_window", privateTravelWindow),
		slog.String("workflow_id", "workflow-1"),
	)
	assertPrivateAbsent(t, output.String())
	if !strings.Contains(output.String(), "workflow-1") {
		t.Fatalf("safe workflow id removed: %s", output.String())
	}
}

func assertScrubbedTrace(t *testing.T) {
	t.Helper()
	exporter := &capturedSpans{}
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(NewScrubbedSpanExporter(exporter)))
	ctx, span := provider.Tracer("gridos").Start(context.Background(), privateSite)
	span.SetAttributes(
		attribute.String("site_id", privateSite),
		attribute.String("command_credential", privateCredential),
		attribute.String("travel_window", privateTravelWindow),
		attribute.String("workflow_id", "workflow-1"),
	)
	span.AddEvent(privateTravelWindow, trace.WithAttributes(attribute.String("site_id", privateSite)))
	span.End()
	if err := provider.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(exporter.spans)
	if err != nil {
		t.Fatal(err)
	}
	assertPrivateAbsent(t, string(encoded))
	if len(exporter.spans) != 1 || exporter.spans[0].Name() != "gridos.operation" {
		t.Fatalf("unexpected exported spans: %d", len(exporter.spans))
	}
	if len(exporter.spans[0].Attributes()) != 1 || exporter.spans[0].Attributes()[0].Value.AsString() != "workflow-1" {
		t.Fatalf("safe workflow attribute missing: %+v", exporter.spans[0].Attributes())
	}
}

func assertScrubbedAnalytics(t *testing.T) {
	t.Helper()
	sink := &capturedAnalytics{}
	wrapped, err := NewScrubbedAnalyticsSink(sink, []byte("test-only-scrub-key"))
	if err != nil {
		t.Fatal(err)
	}
	record := analytics.Record{
		ID:   privateSite,
		Kind: analytics.DispatchFact,
		Provenance: analytics.Provenance{
			Class: "SIMULATED", SourceID: privateSite, SourceURI: privateTravelWindow,
			ObservedAt: time.Date(2026, 8, 12, 18, 0, 0, 0, time.UTC),
			IngestedAt: time.Date(2026, 8, 12, 18, 1, 0, 0, time.UTC), SchemaVersion: "v1",
		},
		Payload: json.RawMessage(`{"site_id":"site-private-123","command_credential":"credential-private-456","travel_window":"travel-window-private-789","count":5}`),
	}
	if err := wrapped.Write(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(sink.record)
	if err != nil {
		t.Fatal(err)
	}
	assertPrivateAbsent(t, string(encoded))
	if !strings.Contains(string(encoded), `"count":5`) {
		t.Fatalf("safe aggregate count removed: %s", encoded)
	}
}

func assertPrivateAbsent(t *testing.T, output string) {
	t.Helper()
	for _, private := range []string{privateSite, privateCredential, privateTravelWindow} {
		if strings.Contains(output, private) {
			t.Fatalf("private value reached exporter: %s", output)
		}
	}
}

type capturedSpans struct {
	spans []sdktrace.ReadOnlySpan
}

func (c *capturedSpans) ExportSpans(_ context.Context, spans []sdktrace.ReadOnlySpan) error {
	c.spans = append(c.spans, spans...)
	return nil
}

func (c *capturedSpans) Shutdown(context.Context) error {
	return nil
}

type capturedAnalytics struct {
	record analytics.Record
}

func (c *capturedAnalytics) Write(_ context.Context, record analytics.Record) error {
	c.record = record
	return nil
}
