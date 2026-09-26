package failures

import (
	"testing"
	"time"
)

func TestHoustonTwentyUsesAffectedSchedules(t *testing.T) {
	at := time.Date(2026, time.August, 12, 18, 15, 0, 0, time.UTC)
	devices := []Device{
		{ID: "a", Region: "LZ_HOUSTON", ScheduledKW: 1},
		{ID: "b", Region: "LZ_HOUSTON", ScheduledKW: 2},
		{ID: "c", Region: "LZ_HOUSTON", ScheduledKW: 4},
		{ID: "d", Region: "LZ_HOUSTON", ScheduledKW: 8},
		{ID: "e", Region: "LZ_HOUSTON", ScheduledKW: 32},
	}
	engine, err := NewEngine(Scenario{
		Seed: 41, Start: at, Tick: time.Minute,
		Injections: []Injection{{At: at, Kind: PartialRegionOutage}},
	}, devices)
	if err != nil {
		t.Fatal(err)
	}
	effects := engine.Advance(at)
	if len(effects) != 1 || len(effects[0].DeviceIDs) != 1 {
		t.Fatalf("effects=%+v", effects)
	}
	schedules := map[string]float64{"a": 1, "b": 2, "c": 4, "d": 8, "e": 32}
	want := schedules[effects[0].DeviceIDs[0]] / 1000
	if effects[0].LostMW != want {
		t.Fatalf("lost MW=%v want=%v", effects[0].LostMW, want)
	}
	if effects[0].LostMW == 0.2*47/1000 {
		t.Fatalf("lost MW used fleet percentage: %v", effects[0].LostMW)
	}
}
