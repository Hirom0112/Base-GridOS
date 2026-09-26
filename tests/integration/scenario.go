package integration

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

type Scenario struct {
	Name       string `yaml:"name"`
	Provenance string `yaml:"provenance"`
	Clock      struct {
		Seed            int64     `yaml:"seed"`
		StartAt         time.Time `yaml:"start_at"`
		IntervalSeconds int       `yaml:"interval_seconds"`
	} `yaml:"clock"`
	Fleet struct {
		Path string `yaml:"path"`
		Size int    `yaml:"size"`
	} `yaml:"fleet"`
	Event struct {
		Region   string    `yaml:"region"`
		StartAt  time.Time `yaml:"start_at"`
		EndAt    time.Time `yaml:"end_at"`
		TargetMW float64   `yaml:"target_mw"`
		Boundary string    `yaml:"boundary"`
	} `yaml:"event"`
	Injections []Injection `yaml:"injections"`
	Expected   struct {
		FinalEventState         string   `yaml:"final_event_state"`
		ReserveViolations       int      `yaml:"reserve_violations"`
		AllowsShortfall         bool     `yaml:"allows_shortfall"`
		RequiredRecoveryActions []string `yaml:"required_recovery_actions"`
	} `yaml:"expected"`
}

type Injection struct {
	At   time.Time `yaml:"at"`
	Kind string    `yaml:"kind"`
}

type FleetDevice struct {
	DeviceID                 string  `json:"device_id"`
	SiteID                   string  `json:"site_id"`
	LoadZone                 string  `json:"load_zone"`
	ReservePreferencePercent float64 `json:"reserve_preference_percent"`
	UsableEnergyKWh          float64 `json:"usable_energy_kwh"`
	MaxDischargeKW           float64 `json:"max_discharge_kw"`
}

func loadScenario(t *testing.T, root, name string) Scenario {
	t.Helper()
	file, err := os.Open(filepath.Join(root, "testdata/scenarios", name+".yaml"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	var scenario Scenario
	if err = decoder.Decode(&scenario); err != nil {
		t.Fatal(err)
	}
	if scenario.Name != name || scenario.Provenance != "SIMULATED" || !scenario.Event.EndAt.After(scenario.Event.StartAt) {
		t.Fatalf("scenario %s = %#v", name, scenario)
	}
	return scenario
}

func (scenario Scenario) duration() time.Duration {
	return scenario.Event.EndAt.Sub(scenario.Event.StartAt)
}

func (stack *stack) cohort(t *testing.T, size int) []FleetDevice {
	t.Helper()
	file, err := os.Open(filepath.Join(stack.root, stack.scenario.Fleet.Path))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	devices := make([]FleetDevice, 0, size)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() && len(devices) < size {
		var device FleetDevice
		if err = json.Unmarshal(scanner.Bytes(), &device); err != nil {
			t.Fatal(err)
		}
		if device.LoadZone == stack.scenario.Event.Region {
			devices = append(devices, device)
		}
	}
	if err = scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(devices) < size {
		t.Fatalf("fleet %s holds %d devices in %s, cohort needs %d", stack.scenario.Fleet.Path, len(devices), stack.scenario.Event.Region, size)
	}
	return devices
}
