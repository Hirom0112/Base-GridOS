package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/gateway"
	"github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/telemetry"
)

func startGatewayMetrics(ctx context.Context, address string, store *gateway.Store, fleet *telemetry.Fleet) error {
	if address == "" {
		return nil
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	server := &http.Server{Handler: gatewayMetricsHandler(store, fleet), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("gateway metrics shutdown: %v", err)
		}
	}()
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("gateway metrics: %v", err)
		}
	}()
	return nil
}

func gatewayMetricsHandler(store *gateway.Store, fleet *telemetry.Fleet) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		ctx, cancel := context.WithTimeout(request.Context(), 2*time.Second)
		defer cancel()
		buffered, err := store.BufferedObservationCount(ctx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var failures uint64
		if fleet != nil {
			failures = fleet.PublishFailures()
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = fmt.Fprintf(w, "# TYPE gridos_gateway_up gauge\ngridos_gateway_up 1\n# TYPE gridos_gateway_publish_failures_total counter\ngridos_gateway_publish_failures_total %d\n# TYPE gridos_gateway_buffered_rows gauge\ngridos_gateway_buffered_rows %d\n", failures, buffered)
	})
}
