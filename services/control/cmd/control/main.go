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

	"connectrpc.com/connect"
	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
	controlapi "github.com/Hirom0112/Base-GridOS/services/control/internal/api"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/ingest"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/storage"
	storagepublisher "github.com/Hirom0112/Base-GridOS/services/control/internal/storage/publisher"
	"github.com/jackc/pgx/v5/pgxpool"
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
		telemetryToken = "Bearer local-gateway"
	}
	service := controlapi.NewService(controlapi.NewPostgresEventStore(pool), twin, sites, time.Now)
	service.SetReportSource(controlapi.NewPostgresReportSource(pool))
	decisionAddress := environment("GRIDOS_DECISION_ADDR", "http://localhost:8082")
	gatewayAddress := environment("GRIDOS_GATEWAY_ADDR", "http://localhost:8081")
	client := h2Client()
	publisher := storagepublisher.New(storagepublisher.Config{
		Pool: pool, Client: gridosv1connect.NewCommandServiceClient(client, gatewayAddress, connect.WithGRPC()), AuthorizationToken: telemetryToken,
		BatchSize: 100, LeaseDuration: 5 * time.Second, AcknowledgementTimeout: 5 * time.Second, Now: time.Now, Interval: uncertainInterval,
	})
	events := controlapi.NewPostgresEventStore(pool)
	service.SetDispatcher(&controlapi.Dispatcher{
		Events: events, Snapshots: controlapi.NewFleetSnapshotter(twin, sites, time.Now),
		Optimizer: controlapi.NewConnectOptimizer(gridosv1connect.NewOptimizationServiceClient(client, decisionAddress, connect.WithGRPC())),
		Safety:    controlapi.IndependentSafetyGate{}, Approval: controlapi.NewStoredApprovalGate(events),
		Commands: controlapi.NewCommandPipeline(pool, publisher), Now: time.Now,
	})
	if !service.RuntimeReady() {
		log.Fatal("control runtime is incomplete")
	}
	telemetry := ingest.NewService(storage.NewTelemetryStore(pool), telemetryTwin, time.Now)
	server := &http.Server{Addr: address, Handler: controlapi.NewControlHandler(service, telemetry, telemetryToken), ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second}
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

func h2Client() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)
	transport.Protocols = protocols
	return &http.Client{Transport: transport, Timeout: 10 * time.Second}
}

func uncertainInterval(command storage.ClaimedCommand, now time.Time) storage.FeasiblePowerInterval {
	lower := min(0, command.SetpointKW)
	upper := max(0, command.SetpointKW)
	return storage.FeasiblePowerInterval{
		DeviceID: command.DeviceID, IntervalBegin: now, IntervalEnd: command.ExpiresAt,
		LowerKW: lower, UpperKW: upper, PossiblyAcceptedCommandID: command.CommandID,
		PossiblyAcceptedSetpointKW: command.SetpointKW, PossiblyAcceptedEffectiveAt: command.EffectiveAt,
		PossiblyAcceptedExpiresAt: command.ExpiresAt, FreshTelemetryObservedAt: now,
		DerivedAt: now, CorrelationID: command.CorrelationID,
	}
}
