package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
	controlapi "github.com/Hirom0112/Base-GridOS/services/control/internal/api"
	apicontext "github.com/Hirom0112/Base-GridOS/services/control/internal/api/context"
	apievents "github.com/Hirom0112/Base-GridOS/services/control/internal/api/events"
	apigeo "github.com/Hirom0112/Base-GridOS/services/control/internal/api/geo"
	apimember "github.com/Hirom0112/Base-GridOS/services/control/internal/api/member"
	apireplay "github.com/Hirom0112/Base-GridOS/services/control/internal/api/replay"
	apireport "github.com/Hirom0112/Base-GridOS/services/control/internal/api/report"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/dispatch"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/ingest"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/observability"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/replay"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.temporal.io/sdk/client"
)

const (
	LOCAL_GATEWAY_CREDENTIAL        = "Bearer local-gateway"
	LOCAL_GATEWAY_CREDENTIAL_STATUS = "STUBBED"
	LOCAL_STEP_UP_STATUS            = "STUBBED"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	slog.SetDefault(slog.New(observability.NewScrubbedLogHandler(slog.NewJSONHandler(os.Stdout, nil))))
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
	if key := os.Getenv("GRIDOS_STEP_UP_KEY"); key == "" {
		log.Printf("%s: dispatch approval and emergency stop step-up is disabled until GRIDOS_STEP_UP_KEY is set", LOCAL_STEP_UP_STATUS)
	} else if len(key) < 32 {
		log.Fatal("GRIDOS_STEP_UP_KEY must contain at least 32 bytes")
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
	tracer, err := startObservability(ctx, pool, twin, telemetryTwin, sites)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = tracer.Shutdown(context.Background()) }()
	if err = fleet.SeedSimulatedMemberSites(ctx, pool, sites); err != nil {
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
	eventSource := apievents.NewPostgresSource(pool, service, sites, time.Now, 30*time.Second)
	events := apievents.NewService(eventSource, 250*time.Millisecond)
	transport := http.DefaultTransport.(*http.Transport).Clone()
	protocols := new(http.Protocols)
	protocols.SetUnencryptedHTTP2(true)
	transport.Protocols = protocols
	decisionClient := gridosv1connect.NewOptimizationServiceClient(&http.Client{Transport: transport, Timeout: 20 * time.Second}, environment("GRIDOS_DECISION_ADDR", "http://localhost:50061"), connect.WithGRPC())
	mux := http.NewServeMux()
	mux.Handle(gridosv1connect.NewContextServiceHandler(apicontext.NewService(environment("GRIDOS_PUBLIC_CONTEXT_DIR", "testdata/fixtures/public"), time.Now)))
	mux.Handle(gridosv1connect.NewReportServiceHandler(apireport.NewService(controlapi.NewPostgresReportSource(pool))))
	mux.Handle(gridosv1connect.NewGeoServiceHandler(apigeo.NewService(sites, apigeo.PostgresSnapshot(sites, twin, pool), time.Now)))
	mux.Handle(gridosv1connect.NewMemberServiceHandler(apimember.NewService(pool, twin, sites, time.Now)))
	replayPath, replayHandler := gridosv1connect.NewReplayServiceHandler(apireplay.NewService(environment("GRIDOS_REPLAY_DIR", ".local/replay"), replay.PostgresSource{Pool: pool}, eventSource, controlapi.NewConnectOptimizer(decisionClient)))
	mux.Handle(replayPath, replayHandler)
	geoAssets, err := apigeo.AssetHandler(os.DirFS("testdata/fixtures/geo"))
	if err != nil {
		log.Fatal(err)
	}
	mux.Handle("/geo/", geoAssets)
	mux.Handle("/", controlapi.NewControlHandler(service, telemetry, telemetryToken, events))
	server := &http.Server{Addr: address, Handler: mux, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second}
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

func startObservability(ctx context.Context, pool *pgxpool.Pool, twin *fleet.Twin, telemetryTwin *fleet.TelemetryTwin, sites []*gridosv1.AuthorizedSite) (*sdktrace.TracerProvider, error) {
	exporter, err := stdouttrace.New(stdouttrace.WithWriter(os.Stdout))
	if err != nil {
		return nil, err
	}
	tracer := observability.NewTracerProvider(exporter)
	otel.SetTracerProvider(tracer)
	if address := os.Getenv("GRIDOS_CONTROL_METRICS_ADDRESS"); address != "" {
		snapshotter := controlapi.NewDurableFleetSnapshotter(pool, twin, telemetryTwin, sites, time.Now)
		var scrapeMu sync.Mutex
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			scrapeMu.Lock()
			defer scrapeMu.Unlock()
			count, err := storage.UncertainCommandCount(r.Context(), pool)
			if err != nil {
				http.Error(w, "metrics unavailable", http.StatusServiceUnavailable)
				return
			}
			total, stale, freshness, err := snapshotter.TelemetryMetrics(r.Context())
			if err != nil {
				http.Error(w, "metrics unavailable", http.StatusServiceUnavailable)
				return
			}
			_ = observability.ProcessMetrics.SetUncertainCommands(count)
			_ = observability.ProcessMetrics.SetTelemetryPopulation(total, stale)
			_ = observability.ProcessMetrics.SetTelemetryFreshness(freshness)
			observability.ProcessMetrics.Handler().ServeHTTP(w, r)
		})
		if err := observability.ServeMetrics(ctx, address, handler); err != nil {
			_ = tracer.Shutdown(ctx)
			return nil, err
		}
	}
	return tracer, nil
}

func environment(name, fallback string) string {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	return value
}
