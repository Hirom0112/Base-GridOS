package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
)

func TestTelemetryPublishTimeoutFitsInsideOneDemoCadence(t *testing.T) {
	if telemetryPublishTimeout <= 0 || telemetryPublishTimeout >= 15*time.Second {
		t.Fatalf("telemetryPublishTimeout=%s", telemetryPublishTimeout)
	}
}

func TestTelemetryPublishAgainstHungControlPlaneReturnsWithinTimeout(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		<-release
	}))
	t.Cleanup(server.Close)
	t.Cleanup(func() { close(release) })
	publisher := newTelemetryPublisher(server.URL, "gateway-1", "Bearer token", 100*time.Millisecond)
	started := time.Now()
	err := publisher.Publish(context.Background(), &gridosv1.TelemetryObservation{ObservationId: "observation-1"})
	elapsed := time.Since(started)
	if err == nil {
		t.Fatal("publish against a hung control plane succeeded")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("publish returned after %s", elapsed)
	}
}
