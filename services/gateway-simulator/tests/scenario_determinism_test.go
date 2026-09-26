package tests

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	scenariorunner "github.com/Hirom0112/Base-GridOS/services/gateway-simulator/cmd/scenario"
)

func TestScenarioDeterminism(t *testing.T) {
	directory := t.TempDir()
	fleetPath := filepath.Join(directory, "fleet.jsonl")
	if err := os.WriteFile(fleetPath, []byte("{\"device_id\":\"device-2\"}\n{\"device_id\":\"device-1\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	scenarioPath := filepath.Join(directory, "scenario.yaml")
	payload := fmt.Sprintf("name: deterministic\nclock:\n  seed: 20260926\n  start_at: 2026-08-12T16:00:00-05:00\n  interval_seconds: 300\nfleet:\n  path: %s\n  size: 2\ninjections:\n  - at: 2026-08-12T16:05:00-05:00\n    kind: OFFLINE_DEVICES\n", fleetPath)
	if err := os.WriteFile(scenarioPath, []byte(payload), 0o600); err != nil {
		t.Fatal(err)
	}
	first, err := scenariorunner.TelemetryHashes(scenarioPath)
	if err != nil {
		t.Fatal(err)
	}
	second, err := scenariorunner.TelemetryHashes(scenarioPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 || !reflect.DeepEqual(first, second) {
		t.Fatalf("first=%v second=%v", first, second)
	}
}
