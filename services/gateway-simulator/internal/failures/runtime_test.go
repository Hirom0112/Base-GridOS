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
