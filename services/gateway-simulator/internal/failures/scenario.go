package failures

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Kind string

type Scope string

const (
	Scheduled   Scope = "scheduled"
	NextCommand Scope = "next_command"
)

const (
	OfflineDevices      Kind = "OFFLINE_DEVICES"
	DelayedGateway      Kind = "DELAYED_GATEWAY"
	DelayedTelemetry    Kind = "DELAYED_TELEMETRY"
	DroppedMessages     Kind = "DROPPED_MESSAGES"
	DuplicatedMessages  Kind = "DUPLICATED_MESSAGES"
	GatewayRestart      Kind = "GATEWAY_RESTART"
	PartialRegionOutage Kind = "PARTIAL_REGION_OUTAGE"
	BadForecasts        Kind = "BAD_FORECASTS"
	HotBatteries        Kind = "HOT_BATTERIES"
	StaleState          Kind = "STALE_STATE"
	OptimizerTimeout    Kind = "OPTIMIZER_TIMEOUT"
)

type Injection struct {
	At     time.Time
	Kind   Kind
	Scope  Scope
	Region string
}

type Scenario struct {
	Name       string
	Seed       int64
	Start      time.Time
	Tick       time.Duration
	FleetPath  string
	FleetSize  int
	EventStart time.Time
	EventEnd   time.Time
	Injections []Injection
}

func LoadScenario(path string) (scenario Scenario, err error) {
	file, err := os.Open(path)
	if err != nil {
		return Scenario{}, err
	}
	defer func() {
		err = errors.Join(err, file.Close())
	}()
	section := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if err := parseScenarioLine(&scenario, &section, scanner.Text()); err != nil {
			return Scenario{}, err
		}
	}
	if err := scanner.Err(); err != nil {
		return Scenario{}, err
	}
	if scenario.Name == "" || scenario.Start.IsZero() || scenario.Tick <= 0 || scenario.FleetPath == "" || scenario.FleetSize <= 0 {
		return Scenario{}, errors.New("scenario name, clock, and fleet are required")
	}
	return scenario, nil
}

func parseScenarioLine(scenario *Scenario, section *string, line string) error {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return nil
	}
	if !strings.HasPrefix(line, " ") {
		*section = strings.TrimSuffix(trimmed, ":")
		if key, value, found := splitField(trimmed); found && key == "name" {
			scenario.Name = value
		}
		return nil
	}
	key, value, found := splitField(strings.TrimPrefix(trimmed, "- "))
	if !found {
		return nil
	}
	switch *section {
	case "clock":
		return parseClockField(scenario, key, value)
	case "fleet":
		return parseFleetField(scenario, key, value)
	case "event":
		if key == "start_at" {
			var err error
			scenario.EventStart, err = time.Parse(time.RFC3339Nano, value)
			if err != nil {
				return fmt.Errorf("event start: %w", err)
			}
		}
		if key == "end_at" {
			var err error
			scenario.EventEnd, err = time.Parse(time.RFC3339Nano, value)
			if err != nil {
				return fmt.Errorf("event end: %w", err)
			}
		}
		return nil
	case "injections":
		return parseInjectionField(scenario, key, value)
	default:
		return nil
	}
}

func (scenario Scenario) RetimeLive(anchor time.Time, window, cadence time.Duration) (Scenario, error) {
	if anchor.IsZero() || window < cadence || cadence <= 0 || !scenario.EventEnd.After(scenario.EventStart) {
		return Scenario{}, errors.New("live scenario requires an event window and positive cadence")
	}
	originalWindow := scenario.EventEnd.Sub(scenario.EventStart)
	scenario.Injections = append([]Injection(nil), scenario.Injections...)
	for index := range scenario.Injections {
		offset := scenario.Injections[index].At.Sub(scenario.EventStart)
		if offset < 0 || offset > originalWindow {
			return Scenario{}, errors.New("live injection outside event window")
		}
		mapped := time.Duration(float64(offset) / float64(originalWindow) * float64(window))
		slots := mapped / cadence
		if mapped%cadence != 0 {
			slots++
		}
		if slots*cadence > window {
			slots = window / cadence
		}
		scenario.Injections[index].At = anchor.Add(slots * cadence)
	}
	scenario.Start = anchor
	scenario.Tick = cadence
	scenario.EventStart = anchor
	scenario.EventEnd = anchor.Add(window)
	return scenario, nil
}

func splitField(line string) (string, string, bool) {
	key, value, found := strings.Cut(line, ":")
	return strings.TrimSpace(key), strings.Trim(strings.TrimSpace(value), `"'`), found
}

func parseClockField(scenario *Scenario, key, value string) error {
	var err error
	switch key {
	case "seed":
		scenario.Seed, err = strconv.ParseInt(value, 10, 64)
	case "start_at":
		scenario.Start, err = time.Parse(time.RFC3339Nano, value)
	case "interval_seconds":
		var seconds int64
		seconds, err = strconv.ParseInt(value, 10, 64)
		scenario.Tick = time.Duration(seconds) * time.Second
	}
	if err != nil {
		return fmt.Errorf("clock %s: %w", key, err)
	}
	return nil
}

func parseFleetField(scenario *Scenario, key, value string) error {
	if key == "path" {
		scenario.FleetPath = value
		return nil
	}
	if key != "size" {
		return nil
	}
	size, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("fleet size: %w", err)
	}
	scenario.FleetSize = size
	return nil
}

func parseInjectionField(scenario *Scenario, key, value string) error {
	if key == "at" {
		at, err := time.Parse(time.RFC3339Nano, value)
		if err != nil {
			return fmt.Errorf("injection time: %w", err)
		}
		scenario.Injections = append(scenario.Injections, Injection{At: at})
		return nil
	}
	if key != "kind" && key != "scope" && key != "region" {
		return nil
	}
	if len(scenario.Injections) == 0 {
		return errors.New("injection kind requires a time")
	}
	injection := &scenario.Injections[len(scenario.Injections)-1]
	switch key {
	case "scope":
		injection.Scope = Scope(value)
	case "region":
		injection.Region = value
	default:
		injection.Kind = Kind(value)
	}
	return nil
}
