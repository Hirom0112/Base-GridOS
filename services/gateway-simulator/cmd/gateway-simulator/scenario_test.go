package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScenarioFlagConfiguresFleetAndClock(t *testing.T) {
	directory := t.TempDir()
	fleetPath := filepath.Join(directory, "fleet.jsonl")
	if err := os.WriteFile(fleetPath, []byte("{\"device_id\":\"a\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	scenarioPath := filepath.Join(directory, "scenario.yaml")
	payload := fmt.Sprintf("name: flag\nclock:\n  seed: 41\n  start_at: 2026-08-12T16:00:00-05:00\n  interval_seconds: 300\nfleet:\n  path: %s\n  size: 1\ninjections: []\n", fleetPath)
	if err := os.WriteFile(scenarioPath, []byte(payload), 0o600); err != nil {
		t.Fatal(err)
	}
	configuration, err := parseConfig([]string{"--scenario", scenarioPath, "--gateway-id", "gateway-1"})
	if err != nil {
		t.Fatal(err)
	}
	if configuration.fleetPath != fleetPath || configuration.scenarioPath != scenarioPath {
		t.Fatalf("configuration=%+v", configuration)
	}
	want := time.Date(2026, time.August, 12, 16, 0, 0, 0, time.FixedZone("", -5*60*60))
	if !configuration.scenarioStart.Equal(want) {
		t.Fatalf("scenario start=%s want=%s", configuration.scenarioStart, want)
	}
}
