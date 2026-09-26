package fleet_test

import (
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet/geo"
)

func TestH3CapacityAndAvailability(t *testing.T) {
	now := time.Unix(100, 0)
	sites := []*gridosv1.AuthorizedSite{
		h3Site("site-1", "cell-1", 10, 5),
		h3Site("site-2", "cell-1", 8, 3),
	}
	states := []fleet.SiteState{
		{SiteID: "site-1", ObservedAt: now.Add(-time.Second), OperatingState: fleet.OnGrid, Availability: fleet.Online, DispatchableKW: 4, ReserveKWh: 2},
		{SiteID: "site-2", ObservedAt: now.Add(-6 * time.Second), OperatingState: fleet.OnGrid, Availability: fleet.Stale, DispatchableKW: 3, ReserveKWh: 1},
	}

	cells := geo.Aggregate(sites, states, now)
	if len(cells) != 1 {
		t.Fatalf("cells = %d, want 1", len(cells))
	}
	cell := cells[0]
	if cell.Cell != "cell-1" || cell.SiteCount != 2 || cell.InstalledMW != 0.008 || cell.InstalledMWh != 0.018 {
		t.Fatalf("identity and installed capacity = %#v", cell)
	}
	if cell.DispatchableMW != 0.004 || cell.ReservedMWh != 0.003 {
		t.Fatalf("dispatchable and reserved capacity = %#v", cell)
	}
	if cell.OperatingCounts[fleet.OnGrid] != 2 || cell.AvailabilityCounts[fleet.Online] != 1 || cell.AvailabilityCounts[fleet.Stale] != 1 {
		t.Fatalf("state counts = %#v, %#v", cell.OperatingCounts, cell.AvailabilityCounts)
	}
	if cell.Freshness != 6*time.Second {
		t.Fatalf("freshness = %s, want 6s", cell.Freshness)
	}
}

func h3Site(siteID, cell string, energyKWh, dischargeKW float64) *gridosv1.AuthorizedSite {
	return &gridosv1.AuthorizedSite{
		Site: &gridosv1.Site{SiteId: siteID, H3Cell: cell},
		Devices: []*gridosv1.Device{{
			DeviceId: siteID + "-device",
			SiteId:   siteID,
			BatteryParameters: &gridosv1.BatteryParameters{
				UsableEnergyKwh: energyKWh,
				MaxDischargeKw:  dischargeKW,
			},
		}},
	}
}
