package observability

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestGatewayTraceKeepsIdentityAndScrubsPrivateFields(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	provider, err := NewTraceProvider(&output)
	if err != nil {
		t.Fatal(err)
	}
	ctx, span, err := Start(context.Background(), provider, "correlation-1", "workflow-1", "gateway.command", map[string]string{
		"site_id": "site-private-123", "command_credential": "credential-private-456", "travel_window": "travel-window-private-789",
	})
	if err != nil {
		t.Fatal(err)
	}
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
