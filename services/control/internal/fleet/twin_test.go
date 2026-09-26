package fleet

import (
	"testing"
	"time"
)

func TestTwinKeepsLatestTelemetryAndIgnoresCommands(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	twin := NewTwin(time.Minute)
	latest := SiteState{SiteID: "site-1", ObservedAt: now, OperatingState: OnGrid, DispatchableKW: 5, DispatchableKWh: 8, BackupHoursCurrent: 3, BackupHours750W: 12, Provenance: "simulated"}
	twin.Accept(latest)
	twin.Accept(SiteState{SiteID: "site-1", ObservedAt: now.Add(-time.Second), DispatchableKW: 99})
	twin.CommandIssued("site-1", 4)

	got, ok := twin.Site("site-1", now)
	if !ok || got != latest {
		t.Fatalf("site state = %#v, %v; want latest accepted %#v", got, ok, latest)
	}
}

func TestTwinMarksStaleAndExcludesUnavailableStates(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	twin := NewTwin(time.Minute)
	twin.Accept(SiteState{SiteID: "stale", ObservedAt: now.Add(-time.Minute - time.Nanosecond), OperatingState: OnGrid, DispatchableKW: 9, DispatchableKWh: 10, Provenance: "simulated"})
	twin.Accept(SiteState{SiteID: "outage", ObservedAt: now, OperatingState: OffGridOutage, DispatchableKW: 9, DispatchableKWh: 10, Provenance: "simulated"})
	twin.Accept(SiteState{SiteID: "no-power", ObservedAt: now, OperatingState: OffGridNoHomePower, DispatchableKW: 9, DispatchableKWh: 10, Provenance: "simulated"})
	twin.Accept(SiteState{SiteID: "overcurrent", ObservedAt: now, OperatingState: OffGridOvercurrent, DispatchableKW: 9, DispatchableKWh: 10, Provenance: "simulated"})
	twin.Accept(SiteState{SiteID: "standby", ObservedAt: now, OperatingState: OffGridOvercurrentStandby, DispatchableKW: 9, DispatchableKWh: 10, Provenance: "simulated"})
	twin.Accept(SiteState{SiteID: "missing", ObservedAt: now, OperatingState: TelemetryUnavailable, DispatchableKW: 9, DispatchableKWh: 10, Provenance: "simulated"})

	stale, _ := twin.Site("stale", now)
	if stale.Availability != Stale {
		t.Fatalf("availability = %q, want %q", stale.Availability, Stale)
	}
	aggregate := twin.Aggregate(now)
	if aggregate.DispatchableMW.Value != 0 || aggregate.DispatchableMWh.Value != 0 {
		t.Fatalf("dispatchable = %v MW, %v MWh; want zero", aggregate.DispatchableMW.Value, aggregate.DispatchableMWh.Value)
	}
}

func TestTwinAggregatesFreshOnGridSitesWithMetadata(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	twin := NewTwin(time.Minute)
	twin.Accept(SiteState{SiteID: "one", ObservedAt: now.Add(-10 * time.Second), OperatingState: OnGrid, DispatchableKW: 4000, DispatchableKWh: 7000, BackupHoursCurrent: 2.5, BackupHours750W: 9, Provenance: "simulated"})
	twin.Accept(SiteState{SiteID: "two", ObservedAt: now.Add(-20 * time.Second), OperatingState: OnGrid, DispatchableKW: 6000, DispatchableKWh: 13000, BackupHoursCurrent: 4, BackupHours750W: 15, Provenance: "derived"})

	aggregate := twin.Aggregate(now)
	if aggregate.DispatchableMW.Value != 10 || aggregate.DispatchableMWh.Value != 20 {
		t.Fatalf("dispatchable = %v MW, %v MWh; want 10 MW, 20 MWh", aggregate.DispatchableMW.Value, aggregate.DispatchableMWh.Value)
	}
	if aggregate.BackupHoursCurrent != 6.5 || aggregate.BackupHours750W != 24 {
		t.Fatalf("backup hours = %v, %v; want 6.5, 24", aggregate.BackupHoursCurrent, aggregate.BackupHours750W)
	}
	for _, quantity := range []Quantity{aggregate.DispatchableMW, aggregate.DispatchableMWh} {
		if quantity.Timestamp != now || quantity.Freshness != 20*time.Second || quantity.ProvenanceMix["simulated"] != 1 || quantity.ProvenanceMix["derived"] != 1 {
			t.Fatalf("metadata = %#v; want timestamp, provenance mix, and freshness", quantity)
		}
	}
}
