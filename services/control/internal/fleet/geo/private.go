package geo

import (
	"errors"
	"sort"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet"
	h3 "github.com/uber/h3-go/v4"
)

type privateSite struct {
	site     *gridosv1.AuthorizedSite
	state    fleet.SiteState
	hasState bool
	cell     h3.Cell
	active   bool
}

func AggregatePrivate(sites []*gridosv1.AuthorizedSite, states []fleet.SiteState, activeSites map[string]bool, now time.Time, resolution int) ([]Cell, error) {
	if resolution < 5 || resolution > 7 {
		return nil, errors.New("only H3 resolutions 5 through 7 are available")
	}
	statesBySite := make(map[string]fleet.SiteState, len(states))
	for _, state := range states {
		statesBySite[state.SiteID] = state
	}
	groups := make(map[h3.Cell][]privateSite)
	seen := make(map[string]bool, len(sites))
	for _, site := range sites {
		id := site.GetSite().GetSiteId()
		cell := h3.CellFromString(site.GetSite().GetH3Cell())
		if id == "" || seen[id] || !cell.IsValid() || cell.Resolution() != 7 {
			return nil, errors.New("site needs a unique ID and a resolution-7 H3 cell")
		}
		seen[id] = true
		parent, err := cell.Parent(0)
		if err != nil {
			return nil, err
		}
		state, hasState := statesBySite[id]
		groups[parent] = append(groups[parent], privateSite{site: site, state: state, hasState: hasState, cell: cell, active: activeSites[id]})
	}
	result := make([]Cell, 0)
	for parent, group := range groups {
		cells, err := splitPrivateCells(group, parent, resolution, now)
		if err != nil {
			return nil, err
		}
		result = append(result, cells...)
	}
	sort.Slice(result, func(left, right int) bool { return result[left].Cell < result[right].Cell })
	return result, nil
}

func splitPrivateCells(sites []privateSite, cell h3.Cell, resolution int, now time.Time) ([]Cell, error) {
	if len(sites) < 5 {
		return nil, nil
	}
	if cell.Resolution() == resolution {
		return []Cell{privateCell(sites, cell, now)}, nil
	}
	children := make(map[h3.Cell][]privateSite)
	for _, site := range sites {
		child, err := site.cell.Parent(cell.Resolution() + 1)
		if err != nil {
			return nil, err
		}
		children[child] = append(children[child], site)
	}
	for _, group := range children {
		if len(group) < 5 {
			return []Cell{privateCell(sites, cell, now)}, nil
		}
	}
	result := make([]Cell, 0, len(children))
	for child, group := range children {
		cells, err := splitPrivateCells(group, child, resolution, now)
		if err != nil {
			return nil, err
		}
		result = append(result, cells...)
	}
	return result, nil
}

func privateCell(sites []privateSite, id h3.Cell, now time.Time) Cell {
	cell := Cell{Cell: id.String(), SiteCount: uint64(len(sites)), OperatingCounts: make(map[fleet.OperatingState]uint64), AvailabilityCounts: make(map[fleet.Availability]uint64)}
	for _, site := range sites {
		var energyKWh float64
		for _, device := range site.site.GetDevices() {
			cell.InstalledMW += device.GetBatteryParameters().GetMaxDischargeKw() / 1000
			energyKWh += device.GetBatteryParameters().GetUsableEnergyKwh()
		}
		cell.InstalledMWh += energyKWh / 1000
		if site.active {
			cell.ActiveDispatchCount++
		}
		if !site.hasState {
			cell.SOCUnknownCount++
			continue
		}
		cell.OperatingCounts[site.state.OperatingState]++
		cell.AvailabilityCounts[site.state.Availability]++
		cell.ReservedMWh += site.state.ReserveKWh / 1000
		if site.state.Availability == fleet.Online {
			cell.ConnectedCount++
		}
		if site.state.OperatingState == fleet.OnGrid && site.state.Availability == fleet.Online {
			cell.DispatchableMW += site.state.DispatchableKW / 1000
		}
		if !site.state.ObservedAt.IsZero() {
			cell.Freshness = max(cell.Freshness, max(now.Sub(site.state.ObservedAt), 0))
		}
		switch {
		case energyKWh <= 0:
			cell.SOCUnknownCount++
		case site.state.EnergyKWh/energyKWh < 0.3:
			cell.SOCLowCount++
		case site.state.EnergyKWh/energyKWh < 0.7:
			cell.SOCMediumCount++
		default:
			cell.SOCHighCount++
		}
	}
	return cell
}
