package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1/gridosv1connect"
	scenariorunner "github.com/Hirom0112/Base-GridOS/services/gateway-simulator/cmd/scenario"
	"github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/battery"
	"github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/failures"
	"github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/gateway"
	"github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/observability"
	"github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/protocol"
	"github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/telemetry"
	"go.opentelemetry.io/otel"
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
	scenarioTick     time.Duration
}

type telemetryClock string

const (
	liveClock     telemetryClock = "live"
	scenarioClock telemetryClock = "scenario"
)

type fleetDevice struct {
	DeviceID                 string   `json:"device_id"`
	SiteID                   string   `json:"site_id"`
	LoadZone                 string   `json:"load_zone"`
	WeatherZone              string   `json:"weather_zone"`
	H3Cell                   string   `json:"h3_cell"`
	LoadProfileType          string   `json:"load_profile_type"`
	ReliabilityTrait         string   `json:"reliability_trait"`
	ResiliencePlan           string   `json:"resilience_plan"`
	Provenance               string   `json:"provenance"`
	Cohorts                  []string `json:"cohorts"`
	HasSolar                 bool     `json:"has_solar"`
	HasAutomaticBackup       bool     `json:"has_automatic_backup"`
	UsableEnergyKWh          *float64 `json:"usable_energy_kwh"`
	MaxChargeKW              *float64 `json:"max_charge_kw"`
	MaxDischargeKW           *float64 `json:"max_discharge_kw"`
	ChargeEfficiency         *float64 `json:"charge_efficiency"`
	DischargeEfficiency      *float64 `json:"discharge_efficiency"`
	ReservePreferencePercent *float64 `json:"reserve_preference_percent"`
	HardwareFloorPercent     *float64 `json:"hardware_floor_percent"`
	SimulationSeed           *int64   `json:"simulation_seed"`
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	tracer, err := observability.NewTraceProvider(os.Stdout)
	if err != nil {
		return err
	}
	otel.SetTracerProvider(tracer)
	defer func() { _ = tracer.Shutdown(context.Background()) }()
	configuration, err := parseConfig(os.Args[1:])
	if err != nil {
		return err
	}
	devices, err := loadFleet(configuration.fleetPath)
	if err != nil {
		return err
	}
	runtime, err := failureRuntime(configuration, devices)
	if err != nil {
		return err
	}
	authorizationToken := os.Getenv("GRIDOS_GATEWAY_TOKEN")
	if authorizationToken == "" {
		return errors.New("GRIDOS_GATEWAY_TOKEN is required")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if port := os.Getenv("GRIDOS_GATEWAY_METRICS_PORT"); port != "" {
		listener, err := net.Listen("tcp", ":"+port)
		if err != nil {
			return err
		}
		metrics := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain; version=0.0.4")
			_, _ = io.WriteString(w, "# TYPE gridos_gateway_up gauge\ngridos_gateway_up 1\n")
		}), ReadHeaderTimeout: 5 * time.Second}
		go func() {
			<-ctx.Done()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = metrics.Shutdown(shutdownCtx)
		}()
		go func() {
			if err := metrics.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Printf("gateway metrics: %v", err)
			}
		}()
	}
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
	scenarioNow := func() time.Time {
		ticks := time.Since(startedAt) / configuration.telemetryCadence
		return configuration.scenarioStart.Add(time.Duration(ticks) * configuration.scenarioTick)
	}
	commandHandler := protocol.NewCommandHandler(store, configuration.gatewayID, authorizationToken, scenarioNow)
	commandService := gridosv1connect.CommandServiceHandler(commandHandler)
	if runtime != nil {
		commandService = newRuntimeCommandHandler(commandService, runtime, scenarioNow)
	}
	path, handler := gridosv1connect.NewCommandServiceHandler(commandService)
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
	if err := startTelemetry(ctx, configuration, devices, store, runtime, authorizationToken, telemetryErrors); err != nil {
		return err
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

func failureRuntime(configuration config, devices []fleetDevice) (*failures.Runtime, error) {
	if configuration.scenarioPath == "" {
		return nil, nil
	}
	if _, err := scenariorunner.TelemetryHashes(configuration.scenarioPath); err != nil {
		return nil, err
	}
	scenario, err := failures.LoadScenario(configuration.scenarioPath)
	if err != nil {
		return nil, err
	}
	failureDevices := make([]failures.Device, 0, len(devices))
	for _, device := range devices {
		failureDevices = append(failureDevices, failures.Device{ID: device.DeviceID, Region: device.LoadZone})
	}
	engine, err := failures.NewEngine(scenario, failureDevices)
	if err != nil {
		return nil, err
	}
	return failures.NewRuntime(engine), nil
}

func startTelemetry(ctx context.Context, configuration config, devices []fleetDevice, store *gateway.Store, runtime *failures.Runtime, authorizationToken string, telemetryErrors chan<- error) error {
	if configuration.controlAddress == "" {
		return nil
	}
	profiles, err := telemetry.ReadProfiles(filepath.Join("testdata", "fixtures", "public", "load-profiles", "residential-week.csv"))
	if err != nil {
		return err
	}
	physicalDevices := make([]telemetry.Device, 0, len(devices))
	for _, device := range devices {
		reserve := max(*device.HardwareFloorPercent, *device.ReservePreferencePercent)
		physicalDevices = append(physicalDevices, telemetry.Device{
			DeviceID: device.DeviceID, LoadProfileType: device.LoadProfileType, SimulationSeed: *device.SimulationSeed,
			Parameters: battery.Parameters{
				UsableEnergyKWh: *device.UsableEnergyKWh, HardwareFloorKWh: *device.UsableEnergyKWh * *device.HardwareFloorPercent / 100,
				ReservePercent: reserve, MaxChargeKW: *device.MaxChargeKW, MaxDischargeKW: *device.MaxDischargeKW,
				ChargeEfficiency: *device.ChargeEfficiency, DischargeEfficiency: *device.DischargeEfficiency,
			},
		})
	}
	publisher := telemetry.NewConnectPublisher(gridosv1connect.NewTelemetryServiceClient(http.DefaultClient, configuration.controlAddress), configuration.gatewayID, authorizationToken)
	network, err := failures.NewNetwork(publisher)
	if err != nil {
		return err
	}
	fleet, err := telemetry.NewFleet(store, physicalDevices, profiles, configuration.telemetryCadence, network)
	if err != nil {
		return err
	}
	if err := fleet.SetSourceStep(configuration.scenarioTick); err != nil {
		return err
	}
	if runtime != nil {
		fleet.SetEffects(runtime)
	}
	clock := liveClock
	if configuration.scenarioPath != "" {
		clock = scenarioClock
	}
	go runTelemetry(ctx, fleet, configuration.scenarioStart, configuration.scenarioTick, configuration.telemetryCadence, clock, telemetryErrors)
	return nil
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
	flags.DurationVar(&configuration.telemetryCadence, "cadence", 0, "")
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
	configuration.scenarioTick = scenario.Tick
	if configuration.telemetryCadence == 0 {
		configuration.telemetryCadence = scenario.Tick
	}
	return configuration, nil
}

func runTelemetry(ctx context.Context, fleet *telemetry.Fleet, start time.Time, sourceStep, cadence time.Duration, clock telemetryClock, failures chan<- error) {
	timer := time.NewTimer(cadence)
	select {
	case <-ctx.Done():
		if !timer.Stop() {
			<-timer.C
		}
	case <-timer.C:
		var err error
		switch clock {
		case liveClock:
			err = fleet.RunLive(ctx)
		case scenarioClock:
			err = fleet.Run(ctx, start.Add(sourceStep))
		default:
			err = errors.New("telemetry clock mode is invalid")
		}
		if err != nil && !errors.Is(err, context.Canceled) {
			failures <- err
		}
	}
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
	if configuration.telemetryCadence == 0 {
		configuration.telemetryCadence = 5 * time.Second
	}
	configuration.scenarioTick = configuration.telemetryCadence
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
	decoder.DisallowUnknownFields()
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
		if err := validateFleetDevice(device); err != nil {
			return nil, err
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

func validateFleetDevice(device fleetDevice) error {
	for _, value := range []string{device.DeviceID, device.SiteID, device.LoadZone, device.LoadProfileType, device.H3Cell} {
		if value == "" {
			return errors.New("fleet device identity is required")
		}
	}
	if device.Provenance != "SIMULATED" || device.SimulationSeed == nil {
		return errors.New("fleet provenance and simulation seed are required")
	}
	values := []*float64{device.UsableEnergyKWh, device.MaxChargeKW, device.MaxDischargeKW, device.ChargeEfficiency, device.DischargeEfficiency, device.ReservePreferencePercent, device.HardwareFloorPercent}
	for _, value := range values {
		if value == nil || math.IsNaN(*value) || math.IsInf(*value, 0) {
			return errors.New("fleet numeric fields must be present and finite")
		}
	}
	if *device.UsableEnergyKWh <= 0 {
		return errors.New("fleet energy is invalid")
	}
	for _, power := range []*float64{device.MaxChargeKW, device.MaxDischargeKW} {
		if *power < 0 || *power > *device.UsableEnergyKWh {
			return errors.New("fleet power is invalid")
		}
	}
	for _, efficiency := range []*float64{device.ChargeEfficiency, device.DischargeEfficiency} {
		if *efficiency <= 0 || *efficiency > 1 {
			return errors.New("fleet efficiency is invalid")
		}
	}
	if err := validateReservePercentages(*device.ReservePreferencePercent, *device.HardwareFloorPercent); err != nil {
		return err
	}
	return nil
}

func validateReservePercentages(preference, hardware float64) error {
	if preference < 0 || preference > 100 || hardware <= 0 || hardware > 100 {
		return errors.New("fleet reserve percentages are invalid")
	}
	return nil
}
