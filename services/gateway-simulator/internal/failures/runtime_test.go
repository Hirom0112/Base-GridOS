package failures

import (
	"testing"
	"time"
)

func TestRuntimeTargetsOnlySeededDevices(t *testing.T) {
	start := time.Date(2026, 8, 12, 18, 0, 0, 0, time.UTC)
	scenario := Scenario{
		Seed: 17, Start: start, Tick: time.Minute,
		Injections: []Injection{
			{At: start.Add(time.Minute), Kind: DroppedMessages},
			{At: start.Add(2 * time.Minute), Kind: PartialRegionOutage},
		},
	}
	devices := []Device{
		{ID: "a", Region: "LZ_AEN"},
		{ID: "b", Region: "LZ_HOUSTON"},
		{ID: "c", Region: "LZ_HOUSTON"},
		{ID: "d", Region: "LZ_HOUSTON"},
		{ID: "e", Region: "LZ_HOUSTON"},
		{ID: "f", Region: "LZ_HOUSTON"},
	}
	engine, err := NewEngine(scenario, devices)
	if err != nil {
		t.Fatal(err)
	}
	runtime := NewRuntime(engine)
	runtime.Advance(start.Add(time.Minute))
	affected := 0
	for _, device := range devices {
		if runtime.Affects(string(DroppedMessages), device.ID) {
			affected++
		}
	}
	if affected != 1 {
		t.Fatalf("dropped-message devices=%d", affected)
	}
	runtime.Advance(start.Add(2 * time.Minute))
	regional := 0
	for _, device := range devices {
		if runtime.Affects(string(PartialRegionOutage), device.ID) {
			regional++
			if device.Region != "LZ_HOUSTON" {
				t.Fatalf("affected device %s is outside Houston", device.ID)
			}
		}
	}
	if regional != 1 {
		t.Fatalf("regional devices=%d", regional)
	}
}

func TestScheduledScopeTargetsCommandedDeviceAcrossRetiming(t *testing.T) {
	start := time.Date(2026, 8, 12, 18, 0, 0, 0, time.UTC)
	devices := []Device{{ID: "a", Region: "LZ_AEN"}, {ID: "b", Region: "LZ_AEN"}, {ID: "c", Region: "LZ_AEN"}}
	selected := ""
	for _, shift := range []time.Duration{0, 24 * time.Hour} {
		at := start.Add(shift).Add(time.Minute)
		scenario := Scenario{Seed: 17, Start: start.Add(shift), Tick: time.Minute, Injections: []Injection{{At: at, Kind: DroppedMessages, Scope: Scheduled}}}
		engine, err := NewEngine(scenario, devices)
		if err != nil {
			t.Fatal(err)
		}
		runtime := NewRuntime(engine)
		runtime.RecordCommand(start.Add(shift), "event", "b")
		runtime.RecordCommand(start.Add(shift), "event", "c")
		runtime.RecordCommand(start.Add(shift), "other", "a")
		runtime.RecordCommand(at, "event", "b")
		if runtime.Affects(string(DroppedMessages), "a") {
			t.Fatal("other event's device was selected")
		}
		current := ""
		for _, deviceID := range []string{"b", "c"} {
			if runtime.Affects(string(DroppedMessages), deviceID) {
				current = deviceID
			}
		}
		if current == "" {
			t.Fatal("no commanded device was selected")
		}
		if selected != "" && current != selected {
			t.Fatalf("retiming changed selected device from %s to %s", selected, current)
		}
		selected = current
	}
}

func TestScheduledScopeAffectsTelemetryForOneCommandedEvent(t *testing.T) {
	start := time.Date(2026, 8, 12, 18, 0, 0, 0, time.UTC)
	at := start.Add(time.Minute)
	scenario := Scenario{Seed: 17, Start: start, Tick: time.Minute, Injections: []Injection{{At: at, Kind: DroppedMessages, Scope: Scheduled}}}
	engine, err := NewEngine(scenario, []Device{{ID: "scheduled", Region: "LZ_AEN"}, {ID: "idle", Region: "LZ_AEN"}})
	if err != nil {
		t.Fatal(err)
	}
	runtime := NewRuntime(engine)
	runtime.RecordCommand(start, "event", "scheduled")
	runtime.Advance(at)
	if !runtime.Affects(string(DroppedMessages), "scheduled") {
		t.Fatal("telemetry did not receive the scheduled event's fault")
	}
	if runtime.Affects(string(DroppedMessages), "idle") {
		t.Fatal("telemetry fault selected an idle device")
	}
}

func TestScheduledScopeAffectsTelemetryAcrossActiveEvents(t *testing.T) {
	start := time.Date(2026, 8, 12, 18, 0, 0, 0, time.UTC)
	at := start.Add(time.Minute)
	scenario := Scenario{Seed: 17, Start: start, Tick: time.Minute, Injections: []Injection{{At: at, Kind: DroppedMessages, Scope: Scheduled}}}
	engine, err := NewEngine(scenario, []Device{{ID: "a", Region: "LZ_AEN"}, {ID: "b", Region: "LZ_AEN"}, {ID: "idle", Region: "LZ_AEN"}})
	if err != nil {
		t.Fatal(err)
	}
	runtime := NewRuntime(engine)
	runtime.RecordCommand(start, "event-a", "a")
	runtime.RecordCommand(start, "event-b", "b")
	runtime.Advance(at)
	affected := 0
	for _, deviceID := range []string{"a", "b"} {
		if runtime.Affects(string(DroppedMessages), deviceID) {
			affected++
		}
	}
	if affected != 1 || runtime.Affects(string(DroppedMessages), "idle") {
		t.Fatalf("telemetry affected %d commanded devices and idle=%t", affected, runtime.Affects(string(DroppedMessages), "idle"))
	}
}
