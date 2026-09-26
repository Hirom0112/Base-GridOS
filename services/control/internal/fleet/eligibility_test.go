package fleet

import (
	"testing"
	"time"
)

func TestEligibilityReportsEveryExclusionReason(t *testing.T) {
	at := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	eligible := DeviceEligibility{
		Online:             true,
		Fresh:              true,
		OperatingState:     OnGrid,
		StateOfEnergy:      60,
		EffectiveReserve:   40,
		InRegion:           true,
		ParticipationStart: at.Add(-time.Hour),
		ParticipationEnd:   at.Add(time.Hour),
	}
	cases := []struct {
		name   string
		change func(*DeviceEligibility)
		want   ExclusionReason
	}{
		{"offline", func(d *DeviceEligibility) { d.Online = false }, ExcludedOffline},
		{"stale", func(d *DeviceEligibility) { d.Fresh = false }, ExcludedStale},
		{"islanded", func(d *DeviceEligibility) { d.OperatingState = OffGridOutage }, ExcludedOffGrid},
		{"overcurrent", func(d *DeviceEligibility) { d.OperatingState = OffGridOvercurrent }, ExcludedOvercurrent},
		{"maintenance lock", func(d *DeviceEligibility) { d.MaintenanceLocked = true }, ExcludedMaintenanceLock},
		{"under reserve", func(d *DeviceEligibility) { d.StateOfEnergy = 39 }, ExcludedUnderReserve},
		{"alarm", func(d *DeviceEligibility) { d.HasAlarm = true }, ExcludedAlarm},
		{"outside region", func(d *DeviceEligibility) { d.InRegion = false }, ExcludedOutsideRegion},
		{"before participation", func(d *DeviceEligibility) { d.ParticipationStart = at.Add(time.Second) }, ExcludedOutsideParticipationWindow},
		{"after participation", func(d *DeviceEligibility) { d.ParticipationEnd = at.Add(-time.Second) }, ExcludedOutsideParticipationWindow},
	}
	if got := EvaluateEligibility(eligible, at); !got.Eligible || got.Reason != ExclusionNone {
		t.Fatalf("positive control = %#v, want eligible", got)
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			device := eligible
			test.change(&device)
			got := EvaluateEligibility(device, at)
			if got.Eligible || got.Reason != test.want {
				t.Fatalf("result = %#v, want excluded with %q", got, test.want)
			}
		})
	}
}
