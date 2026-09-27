package failures

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScenarioFileDrivesClockedInjections(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scenario.yaml")
	payload := []byte("name: clocked\nclock:\n  seed: 41\n  start_at: 2026-08-12T16:00:00-05:00\n  interval_seconds: 300\nfleet:\n  path: fleet.jsonl\n  size: 2\ninjections:\n  - at: 2026-08-12T16:05:00-05:00\n    kind: DELAYED_GATEWAY\n")
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	scenario, err := LoadScenario(path)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := NewEngine(scenario, []Device{{ID: "a", Region: "LZ_AEN"}, {ID: "b", Region: "LZ_AEN"}})
	if err != nil {
		t.Fatal(err)
	}
	if effects := engine.Advance(scenario.Start); len(effects) != 0 {
		t.Fatalf("early effects=%+v", effects)
	}
	if effects := engine.Advance(scenario.Start.Add(4 * time.Minute)); len(effects) != 0 {
		t.Fatalf("off-clock effects=%+v", effects)
	}
	effects := engine.Advance(scenario.Start.Add(5 * time.Minute))
	if len(effects) != 1 || effects[0].Kind != DelayedGateway {
		t.Fatalf("effects=%+v", effects)
	}
}

func TestScenarioFileReadsScheduledScope(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scenario.yaml")
	payload := []byte("name: scheduled\nclock:\n  seed: 41\n  start_at: 2026-08-12T16:00:00-05:00\n  interval_seconds: 300\nfleet:\n  path: fleet.jsonl\n  size: 2\ninjections:\n  - at: 2026-08-12T16:05:00-05:00\n    kind: DROPPED_MESSAGES\n    scope: scheduled\n")
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	scenario, err := LoadScenario(path)
	if err != nil {
		t.Fatal(err)
	}
	if scenario.Injections[0].Scope != Scheduled {
		t.Fatalf("scope = %q", scenario.Injections[0].Scope)
	}
}

func TestLiveScenarioRetimesToNextSlotWithoutChangingSource(t *testing.T) {
	start := time.Date(2026, time.August, 12, 18, 0, 0, 0, time.UTC)
	scenario := Scenario{
		Start: start.Add(-2 * time.Hour), Tick: 5 * time.Minute,
		EventStart: start, EventEnd: start.Add(2 * time.Hour),
		Injections: []Injection{{At: start.Add(20 * time.Minute), Kind: DelayedGateway}},
	}
	anchor := time.Date(2026, time.September, 27, 3, 0, 0, 0, time.UTC)
	live, err := scenario.RetimeLive(anchor, 10*time.Minute, 15*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if want := anchor.Add(105 * time.Second); !live.Injections[0].At.Equal(want) {
		t.Fatalf("live injection=%s want=%s", live.Injections[0].At, want)
	}
	if !scenario.Injections[0].At.Equal(start.Add(20 * time.Minute)) {
		t.Fatalf("source injection changed: %s", scenario.Injections[0].At)
	}
}
