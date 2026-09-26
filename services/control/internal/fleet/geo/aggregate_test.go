package geo

import (
	"fmt"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet"
)

func TestAggregatePrivacy(t *testing.T) {
	now := time.Unix(100, 0)
	sites := make([]*gridosv1.AuthorizedSite, 0, 14)
	states := make([]fleet.SiteState, 0, 14)
	active := make(map[string]bool)
	for index := range 14 {
		cell, power, energy, availability := "8726cb9a5ffffff", 9.0, 8.0, fleet.Online
		if index < 3 {
			cell, power, energy = "87489e346ffffff", 5, 2
		} else if index < 9 {
			cell, power, energy, availability = "87489e341ffffff", 7, 5, fleet.Offline
		}
		id := fmt.Sprintf("site-%d", index)
		sites = append(sites, &gridosv1.AuthorizedSite{Site: &gridosv1.Site{SiteId: id, H3Cell: cell}, Devices: []*gridosv1.Device{{BatteryParameters: &gridosv1.BatteryParameters{UsableEnergyKwh: 10, MaxDischargeKw: power}}}})
		states = append(states, fleet.SiteState{SiteID: id, EnergyKWh: energy, Availability: availability, OperatingState: fleet.OnGrid, ObservedAt: now})
		active[id] = index >= 3 && index < 9
	}
	cells, err := AggregatePrivate(sites, states, active, now, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(cells) != 2 {
		t.Fatalf("want two privacy-safe cells, got %d", len(cells))
	}
	byID := make(map[string]Cell, len(cells))
	for _, cell := range cells {
		if cell.SiteCount < 5 {
			t.Fatalf("small cell leaked: %+v", cell)
		}
		byID[cell.Cell] = cell
	}
	merged := byID["86489e347ffffff"]
	if merged.SiteCount != 9 || merged.InstalledMW != 0.057 || merged.SOCLowCount != 3 || merged.SOCMediumCount != 6 || merged.ConnectedCount != 3 || merged.ActiveDispatchCount != 6 {
		t.Fatalf("incorrect merged metrics: %+v", merged)
	}
	separate := byID["8726cb9a5ffffff"]
	if separate.SiteCount != 5 || separate.SOCHighCount != 5 || separate.InstalledMW != 0.045 {
		t.Fatalf("full sibling must remain visible: %+v", separate)
	}
	if _, err := AggregatePrivate(sites, states, active, now, 8); err == nil {
		t.Fatal("resolution 8 must be unavailable without exact site cells")
	}
}
