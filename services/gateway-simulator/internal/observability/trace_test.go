package observability

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/attribute"
)

func TestGatewayTraceKeepsIdentityAndScrubsPrivateFields(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	provider, err := NewTraceProvider(&output)
	if err != nil {
		t.Fatal(err)
	}
	ctx, span := provider.Tracer("gridos.gateway").Start(context.Background(), "gateway.command")
	span.SetAttributes(
		attribute.String("correlation_id", "correlation-1"), attribute.String("workflow_id", "workflow-1"),
		attribute.String("site_id", "site-private-123"), attribute.String("command_credential", "credential-private-456"), attribute.String("travel_window", "travel-window-private-789"),
	)
	span.End()
	if err := provider.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	for _, forbidden := range []string{"site-private-123", "credential-private-456", "travel-window-private-789"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("private field exported: %s", text)
		}
	}
	if !strings.Contains(text, "correlation-1") || !strings.Contains(text, "workflow-1") {
		t.Fatalf("trace identity missing: %s", text)
	}
}
