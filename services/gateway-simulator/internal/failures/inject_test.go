package failures

import (
	"reflect"
	"testing"
	"time"
)

func TestEveryInjectionIsSeededAndReproducible(t *testing.T) {
	at := time.Date(2026, time.August, 12, 18, 15, 0, 0, time.FixedZone("CDT", -5*60*60))
	kinds := []Kind{
		OfflineDevices,
		DelayedTelemetry,
		DroppedMessages,
		DuplicatedMessages,
		GatewayRestart,
		PartialRegionOutage,
		BadForecasts,
		HotBatteries,
		StaleState,
		OptimizerTimeout,
	}
	devices := []Device{
		{ID: "a", Region: "LZ_AEN"},
		{ID: "b", Region: "LZ_HOUSTON"},
		{ID: "c", Region: "LZ_HOUSTON"},
	}
	for _, kind := range kinds {
		t.Run(string(kind), func(t *testing.T) {
			scenario := Scenario{Seed: 20260926, Start: at.Add(-time.Minute), Tick: time.Minute, Injections: []Injection{{At: at, Kind: kind}}}
			first, err := NewEngine(scenario, devices)
			if err != nil {
				t.Fatal(err)
			}
			second, err := NewEngine(scenario, devices)
			if err != nil {
				t.Fatal(err)
			}
			firstEffects := first.Advance(at)
			secondEffects := second.Advance(at)
			if len(firstEffects) != 1 || firstEffects[0].Kind != kind {
				t.Fatalf("effects=%+v", firstEffects)
			}
			if !reflect.DeepEqual(firstEffects, secondEffects) {
				t.Fatalf("first=%+v second=%+v", firstEffects, secondEffects)
			}
			if repeated := first.Advance(at); len(repeated) != 0 {
				t.Fatalf("repeated effects=%+v", repeated)
			}
		})
	}
}
