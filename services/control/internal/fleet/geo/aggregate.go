package geo

import (
	"sort"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet"
)

type Cell struct {
	Cell               string
	SiteCount          uint64
	InstalledMW        float64
	InstalledMWh       float64
	DispatchableMW     float64
	ReservedMWh        float64
	OperatingCounts    map[fleet.OperatingState]uint64
	AvailabilityCounts map[fleet.Availability]uint64
	Freshness          time.Duration
}

func Aggregate(sites []*gridosv1.AuthorizedSite, states []fleet.SiteState, now time.Time) []Cell {
	statesBySite := make(map[string]fleet.SiteState, len(states))
	for _, state := range states {
		statesBySite[state.SiteID] = state
	}
	cells := make(map[string]*Cell)
	for _, site := range sites {
		cellID := site.GetSite().GetH3Cell()
		cell := cells[cellID]
		if cell == nil {
			cell = &Cell{Cell: cellID, OperatingCounts: make(map[fleet.OperatingState]uint64), AvailabilityCounts: make(map[fleet.Availability]uint64)}
			cells[cellID] = cell
		}
		cell.SiteCount++
		for _, device := range site.GetDevices() {
			cell.InstalledMW += device.GetBatteryParameters().GetMaxDischargeKw() / 1000
			cell.InstalledMWh += device.GetBatteryParameters().GetUsableEnergyKwh() / 1000
		}
		state, found := statesBySite[site.GetSite().GetSiteId()]
		if !found {
			continue
		}
		cell.OperatingCounts[state.OperatingState]++
		cell.AvailabilityCounts[state.Availability]++
		cell.ReservedMWh += state.ReserveKWh / 1000
		if !state.ObservedAt.IsZero() {
			cell.Freshness = max(cell.Freshness, max(now.Sub(state.ObservedAt), 0))
		}
		if state.OperatingState == fleet.OnGrid && state.Availability != fleet.Stale && state.Availability != fleet.Offline && state.Availability != fleet.Maintenance {
			cell.DispatchableMW += state.DispatchableKW / 1000
		}
	}
	ordered := make([]Cell, 0, len(cells))
	for _, cell := range cells {
		ordered = append(ordered, *cell)
	}
	sort.Slice(ordered, func(left, right int) bool { return ordered[left].Cell < ordered[right].Cell })
	return ordered
}
