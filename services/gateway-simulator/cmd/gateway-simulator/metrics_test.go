package main

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/gateway"
)

func TestGatewayMetricsExposeBufferedRowsAndPublishFailures(t *testing.T) {
	ctx := context.Background()
	store, err := gateway.Open(ctx, filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := store.BufferObservation(ctx, "buffered-1", []byte("{}")); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	gatewayMetricsHandler(store, nil).ServeHTTP(response, httptest.NewRequest("GET", "/metrics", nil))
	if response.Code != 200 {
		t.Fatalf("metrics status=%d", response.Code)
	}
	for _, line := range []string{"gridos_gateway_up 1", "gridos_gateway_publish_failures_total 0", "gridos_gateway_buffered_rows 1"} {
		if !strings.Contains(response.Body.String(), line) {
			t.Fatalf("metrics missing %q: %s", line, response.Body.String())
		}
	}
}
