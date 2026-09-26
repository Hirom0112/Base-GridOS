package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
	scenariorunner "github.com/Hirom0112/Base-GridOS/services/gateway-simulator/cmd/scenario"
	"github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/failures"
	"github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/gateway"
	"github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/protocol"
	"github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/telemetry"
)

type config struct {
	address          string
	controlAddress   string
	databasePath     string
	fleetPath        string
	gatewayID        string
	scenarioPath     string
	scenarioStart    time.Time
	telemetryCadence time.Duration
}

type fleetDevice struct {
	DeviceID string `json:"device_id"`
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	configuration, err := parseConfig(os.Args[1:])
	if err != nil {
		return err
	}
	devices, err := loadFleet(configuration.fleetPath)
	if err != nil {
		return err
	}
	if configuration.scenarioPath != "" {
		if _, err := scenariorunner.TelemetryHashes(configuration.scenarioPath); err != nil {
			return err
		}
	}
	authorizationToken := os.Getenv("GRIDOS_GATEWAY_TOKEN")
	if authorizationToken == "" {
		return errors.New("GRIDOS_GATEWAY_TOKEN is required")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	store, err := gateway.Open(ctx, configuration.databasePath)
	if err != nil {
		return err
	}
	defer func() {
		if err := store.Close(); err != nil {
			log.Printf("close gateway store: %v", err)
		}
	}()
	startedAt := time.Now()
	scenarioNow := func() time.Time { return configuration.scenarioStart.Add(time.Since(startedAt)) }
	commandHandler := protocol.NewCommandHandler(store, configuration.gatewayID, authorizationToken, scenarioNow)
	path, handler := gridosv1connect.NewCommandServiceHandler(commandHandler)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)
	server := &http.Server{
		Addr:              configuration.address,
		Handler:           mux,
		Protocols:         protocols,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	serveErrors := make(chan error, 1)
	go func() {
		serveErrors <- server.ListenAndServe()
	}()
	telemetryErrors := make(chan error, 1)
	if configuration.controlAddress != "" {
		deviceIDs := make([]string, 0, len(devices))
		for _, device := range devices {
			deviceIDs = append(deviceIDs, device.DeviceID)
		}
		publisher := protocol.NewTelemetryPublisher(
			gridosv1connect.NewTelemetryServiceClient(h2Client(), configuration.controlAddress, connect.WithGRPC()),
			configuration.gatewayID,
			authorizationToken,
		)
		fleet, err := telemetry.NewFleet(store, deviceIDs, configuration.telemetryCadence, publisher)
		if err != nil {
			return err
		}
		go runTelemetry(ctx, fleet, configuration.scenarioStart, configuration.telemetryCadence, telemetryErrors)
	}
	select {
	case err := <-serveErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-telemetryErrors:
		return err
	}
}

func parseConfig(arguments []string) (config, error) {
	flags := flag.NewFlagSet("gateway-simulator", flag.ContinueOnError)
	var configuration config
	var scenarioStart string
	flags.StringVar(&configuration.address, "address", ":8081", "")
	flags.StringVar(&configuration.controlAddress, "control-address", os.Getenv("GRIDOS_CONTROL_ADDR"), "")
	flags.StringVar(&configuration.databasePath, "database", "gateway.db", "")
	flags.StringVar(&configuration.fleetPath, "fleet", "", "")
	flags.StringVar(&configuration.gatewayID, "gateway-id", "", "")
	flags.StringVar(&configuration.scenarioPath, "scenario", "", "")
	flags.StringVar(&scenarioStart, "scenario-start", "", "")
	configuration.telemetryCadence = 5 * time.Second
	if err := flags.Parse(arguments); err != nil {
		return config{}, err
	}
	if configuration.gatewayID == "" {
		return config{}, errors.New("gateway-id is required")
	}
	return configureScenario(configuration, scenarioStart)
}

func configureScenario(configuration config, scenarioStart string) (config, error) {
	if configuration.scenarioPath == "" {
		return configureExplicitScenario(configuration, scenarioStart)
	}
	if configuration.fleetPath != "" || scenarioStart != "" {
		return config{}, errors.New("scenario cannot be combined with fleet or scenario-start")
	}
	scenario, err := failures.LoadScenario(configuration.scenarioPath)
	if err != nil {
		return config{}, err
	}
	configuration.fleetPath = scenario.FleetPath
	configuration.scenarioStart = scenario.Start
	configuration.telemetryCadence = scenario.Tick
	return configuration, nil
}

func runTelemetry(ctx context.Context, fleet *telemetry.Fleet, start time.Time, cadence time.Duration, failures chan<- error) {
	timer := time.NewTimer(cadence)
	select {
	case <-ctx.Done():
		if !timer.Stop() {
			<-timer.C
		}
	case <-timer.C:
		if err := fleet.Run(ctx, start.Add(cadence)); err != nil && !errors.Is(err, context.Canceled) {
			failures <- err
		}
	}
}

func h2Client() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	protocols := new(http.Protocols)
	protocols.SetUnencryptedHTTP2(true)
	transport.Protocols = protocols
	return &http.Client{Transport: transport, Timeout: 10 * time.Second}
}

func configureExplicitScenario(configuration config, scenarioStart string) (config, error) {
	if configuration.fleetPath == "" || scenarioStart == "" {
		return config{}, errors.New("fleet and scenario-start are required")
	}
	parsedStart, err := time.Parse(time.RFC3339Nano, scenarioStart)
	if err != nil {
		return config{}, fmt.Errorf("scenario start: %w", err)
	}
	configuration.scenarioStart = parsedStart
	return configuration, nil
}

func loadFleet(path string) ([]fleetDevice, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("close fleet file: %v", err)
		}
	}()
	decoder := json.NewDecoder(file)
	seen := make(map[string]struct{})
	var devices []fleetDevice
	for {
		var device fleetDevice
		err := decoder.Decode(&device)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if device.DeviceID == "" {
			return nil, errors.New("fleet device identifier is required")
		}
		if _, exists := seen[device.DeviceID]; exists {
			return nil, fmt.Errorf("duplicate fleet device %q", device.DeviceID)
		}
		seen[device.DeviceID] = struct{}{}
		devices = append(devices, device)
	}
	if len(devices) == 0 {
		return nil, errors.New("fleet is empty")
	}
	return devices, nil
}
