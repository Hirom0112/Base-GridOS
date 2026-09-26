package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	controlapi "github.com/Hirom0112/Base-GridOS/services/control/internal/api"
	apievents "github.com/Hirom0112/Base-GridOS/services/control/internal/api/events"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/dispatch"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/ingest"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.temporal.io/sdk/client"
)

const (
	LOCAL_GATEWAY_CREDENTIAL        = "Bearer local-gateway"
	LOCAL_GATEWAY_CREDENTIAL_STATUS = "STUBBED"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	databaseURL := os.Getenv("GRIDOS_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://gridos:gridos@localhost:5432/gridos?sslmode=disable"
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	startupContext, cancelStartup := context.WithTimeout(ctx, 5*time.Second)
	defer cancelStartup()
	if err = pool.Ping(startupContext); err != nil {
		log.Fatal(err)
	}
	address := os.Getenv("GRIDOS_CONTROL_ADDRESS")
	if address == "" {
		address = ":8080"
	}
	fleetPath := os.Getenv("GRIDOS_FLEET")
	if fleetPath == "" {
		fleetPath = "testdata/fleets/austin-5000.jsonl"
	}
	twin := fleet.NewTwin(30 * time.Second)
	sites, telemetryTwin, err := fleet.Load(fleetPath, twin, time.Now())
	if err != nil {
		log.Fatal(err)
	}
	telemetryToken := os.Getenv("GRIDOS_GATEWAY_TOKEN")
	if telemetryToken == "" {
		telemetryToken = LOCAL_GATEWAY_CREDENTIAL
	}
	service := controlapi.NewService(controlapi.NewPostgresEventStore(pool), twin, sites, time.Now)
	service.SetReportSource(controlapi.NewPostgresReportSource(pool))
	temporalClient, err := client.Dial(client.Options{HostPort: environment("TEMPORAL_ADDRESS", client.DefaultHostPort)})
	if err != nil {
		log.Fatal(err)
	}
	defer temporalClient.Close()
	service.SetWorkflowClient(temporalClient, environment("GRIDOS_TASK_QUEUE", dispatch.TaskQueue))
	if !service.RuntimeReady() {
		log.Fatal("control runtime is incomplete")
	}
	telemetry := ingest.NewService(storage.NewTelemetryStore(pool), telemetryTwin, time.Now)
	events := apievents.NewService(apievents.NewPostgresSource(pool, service, sites, time.Now, 30*time.Second), 250*time.Millisecond)
	server := &http.Server{Addr: address, Handler: controlapi.NewControlHandler(service, telemetry, telemetryToken, events), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			log.Printf("control shutdown: %v", err)
		}
	}()
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func environment(name, fallback string) string {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	return value
}
