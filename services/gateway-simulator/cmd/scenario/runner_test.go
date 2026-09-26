package scenario

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestTelemetryHashesAreDeterministic(t *testing.T) {
	directory := t.TempDir()
	fleetPath := filepath.Join(directory, "fleet.jsonl")
	if err := os.WriteFile(fleetPath, []byte("{\"device_id\":\"b\"}\n{\"device_id\":\"a\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	scenarioPath := filepath.Join(directory, "scenario.yaml")
	writeScenario := func(seed int) {
		t.Helper()
		payload := fmt.Sprintf("name: deterministic\nclock:\n  seed: %d\n  start_at: 2026-08-12T16:00:00-05:00\n  interval_seconds: 300\nfleet:\n  path: %s\n  size: 2\ninjections: []\n", seed, fleetPath)
		if err := os.WriteFile(scenarioPath, []byte(payload), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeScenario(41)
	first, err := TelemetryHashes(scenarioPath)
	if err != nil {
		t.Fatal(err)
	}
	second, err := TelemetryHashes(scenarioPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 || !reflect.DeepEqual(first, second) {
		t.Fatalf("first=%v second=%v", first, second)
	}
	writeScenario(42)
	changed, err := TelemetryHashes(scenarioPath)
	if err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(first, changed) {
		t.Fatalf("seed did not affect hashes: %v", changed)
	}
}
