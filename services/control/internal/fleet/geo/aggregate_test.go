package geo

import (
	"fmt"
	"math"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet"
)

func privacyFixture() ([]*gridosv1.AuthorizedSite, []fleet.SiteState, map[string]bool, time.Time) {
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
	return sites, states, active, now
}

func TestAggregatePrivacy(t *testing.T) {
	sites, states, active, now := privacyFixture()
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
	if merged.SiteCount != 9 || math.Abs(merged.InstalledMW-0.057) > 1e-12 || merged.SOCLowCount != 3 || merged.SOCMediumCount != 6 || merged.ConnectedCount != 3 || merged.ActiveDispatchCount != 6 {
		t.Fatalf("incorrect merged metrics: %+v", merged)
	}
	separate := byID["8726cb9a5ffffff"]
	if separate.SiteCount != 5 || separate.SOCHighCount != 5 || math.Abs(separate.InstalledMW-0.045) > 1e-12 {
		t.Fatalf("full sibling must remain visible: %+v", separate)
	}
}

func TestAggregateResolutions(t *testing.T) {
	sites, states, active, now := privacyFixture()
	if _, err := AggregatePrivate(sites, states, active, now, 8); err == nil {
		t.Fatal("resolution 8 must be unavailable without exact site cells")
	}
	for resolution := 5; resolution <= 7; resolution++ {
		cells, err := AggregatePrivate(sites, states, active, now, resolution)
		if err != nil {
			t.Fatal(err)
		}
		var total uint64
		for _, cell := range cells {
			if cell.SiteCount < 5 {
				t.Fatalf("resolution %d disclosed a small cell: %+v", resolution, cell)
			}
			total += cell.SiteCount
		}
		if total != uint64(len(sites)) {
			t.Fatalf("resolution %d counted %d sites, want %d", resolution, total, len(sites))
		}
	}
}

func TestAggregateUnknownSOC(t *testing.T) {
	sites, states, active, now := privacyFixture()
	for index := range states {
		states[index].ObservedAt = time.Time{}
		states[index].EnergyKWh = 0
	}
	cells, err := AggregatePrivate(sites, states, active, now, 5)
	if err != nil {
		t.Fatal(err)
	}
	var unknown, low, medium, high uint64
	for _, cell := range cells {
		unknown += cell.SOCUnknownCount
		low += cell.SOCLowCount
		medium += cell.SOCMediumCount
		high += cell.SOCHighCount
	}
	if unknown != uint64(len(sites)) || low != 0 || medium != 0 || high != 0 {
		t.Fatalf("unobserved state classified as charge: unknown=%d low=%d medium=%d high=%d", unknown, low, medium, high)
	}
}
