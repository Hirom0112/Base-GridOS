package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"connectrpc.com/connect"
	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
	controlapi "github.com/Hirom0112/Base-GridOS/services/control/internal/api"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/dispatch"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/reconciliation"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/replay"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/storage"
	storagepublisher "github.com/Hirom0112/Base-GridOS/services/control/internal/storage/publisher"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, environment("GRIDOS_DATABASE_URL", "postgres://gridos:gridos@localhost:5432/gridos?sslmode=disable"))
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	fleetPath := environment("GRIDOS_FLEET", "testdata/fleets/austin-5000.jsonl")
	sites, twin, telemetry, err := loadFleet(fleetPath)
	if err != nil {
		log.Fatal(err)
	}
	seed, err := fleetSeed(sites)
	if err != nil {
		log.Fatal(err)
	}
	build, _ := debug.ReadBuildInfo()
	version, err := codeVersion(build, os.Getenv("GRIDOS_CODE_VERSION"))
	if err != nil {
		log.Fatal(err)
	}
	httpClient := h2Client()
	events := controlapi.NewPostgresEventStore(pool)
	publisher := storagepublisher.New(storagepublisher.Config{
		Pool: pool, Client: gridosv1connect.NewCommandServiceClient(httpClient, environment("GRIDOS_GATEWAY_ADDR", "http://localhost:8081"), connect.WithGRPC()),
		AuthorizationToken: environment("GRIDOS_GATEWAY_TOKEN", "Bearer local-gateway"), BatchSize: 100,
		LeaseDuration: 5 * time.Second, AcknowledgementTimeout: 5 * time.Second, Now: time.Now, Interval: uncertainInterval,
	})
	dispatcher := &controlapi.Dispatcher{
		Events: events, Snapshots: controlapi.NewDurableFleetSnapshotter(pool, twin, telemetry, sites, time.Now),
		Optimizer: controlapi.NewConnectOptimizer(gridosv1connect.NewOptimizationServiceClient(httpClient, environment("GRIDOS_DECISION_ADDR", "http://localhost:50061"), connect.WithGRPC())),
		Safety:    controlapi.IndependentSafetyGate{}, Approval: controlapi.NewStoredApprovalGate(events),
		Commands: controlapi.NewCommandPipeline(pool, publisher), Now: time.Now,
	}
	activities := &dispatch.Activities{
		Dispatcher: dispatcher, Events: events, Pool: pool, Reports: controlapi.NewPostgresReportSource(pool), Now: time.Now,
		ReplayDirectory: environment("GRIDOS_REPLAY_DIR", ".local/replay"),
		ReplayInput:     replay.Input{Seed: seed, FleetFile: fleetPath, ScenarioFile: os.Getenv("GRIDOS_SCENARIO"), SolverVersion: "highs", FallbackVersion: "1", CodeVersion: version},
	}
	temporalClient, err := client.Dial(client.Options{HostPort: environment("TEMPORAL_ADDRESS", client.DefaultHostPort)})
	if err != nil {
		log.Fatal(err)
	}
	defer temporalClient.Close()
	dispatchWorker := worker.New(temporalClient, environment("GRIDOS_TASK_QUEUE", dispatch.TaskQueue), worker.Options{})
	dispatchWorker.RegisterWorkflow(dispatch.Workflow)
	dispatchWorker.RegisterActivity(activities)
	dispatchWorker.RegisterActivity(&reconciliation.Activities{Pool: pool, Events: storage.NewPostgresEventStore(pool), Now: time.Now, MaxGap: 30 * time.Second})
	if err = dispatchWorker.Run(worker.InterruptCh()); err != nil {
		log.Fatal(err)
	}
}

func codeVersion(build *debug.BuildInfo, fallback string) (string, error) {
	revision, modified := "", false
	if build != nil {
		for _, setting := range build.Settings {
			switch setting.Key {
			case "vcs.revision":
				revision = setting.Value
			case "vcs.modified":
				modified = setting.Value == "true"
			}
		}
	}
	if revision != "" {
		if modified {
			return revision + "+modified", nil
		}
		return revision, nil
	}
	if version := strings.TrimSpace(fallback); version != "" && version != "(devel)" {
		return version, nil
	}
	return "", errors.New("worker code version required")
}

func loadFleet(path string) ([]*gridosv1.AuthorizedSite, *fleet.Twin, *fleet.TelemetryTwin, error) {
	twin := fleet.NewTwin(30 * time.Second)
	sites, telemetry, err := fleet.Load(path, twin, time.Now())
	return sites, twin, telemetry, err
}

func fleetSeed(sites []*gridosv1.AuthorizedSite) (int64, error) {
	var seed int64
	for _, site := range sites {
		provenance := site.GetSite().GetProvenance()
		if provenance == nil || provenance.SimulationSeed == nil || provenance.GetSimulationSeed() == 0 {
			return 0, errors.New("fleet simulation seed required")
		}
		if seed != 0 && provenance.GetSimulationSeed() != seed {
			return 0, errors.New("mixed simulation seeds")
		}
		seed = provenance.GetSimulationSeed()
	}
	if seed == 0 {
		return 0, errors.New("fleet simulation seed required")
	}
	return seed, nil
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
	protocols.SetUnencryptedHTTP2(true)
	transport.Protocols = protocols
	return &http.Client{Transport: transport, Timeout: 10 * time.Second}
}

func uncertainInterval(command storage.ClaimedCommand, now time.Time) storage.FeasiblePowerInterval {
	return storage.FeasiblePowerInterval{
		DeviceID: command.DeviceID, IntervalBegin: now, IntervalEnd: command.ExpiresAt,
		LowerKW: min(0, command.SetpointKW), UpperKW: max(0, command.SetpointKW),
		PossiblyAcceptedCommandID: command.CommandID, PossiblyAcceptedSetpointKW: command.SetpointKW,
		PossiblyAcceptedEffectiveAt: command.EffectiveAt, PossiblyAcceptedExpiresAt: command.ExpiresAt,
		FreshTelemetryObservedAt: now, DerivedAt: now, CorrelationID: command.CorrelationID,
	}
}
