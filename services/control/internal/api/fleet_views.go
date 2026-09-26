package api

import (
	"slices"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet/geo"
)

func installedCapacity(sites []*gridosv1.AuthorizedSite) (float64, float64) {
	var powerKW, energyKWh float64
	for _, site := range sites {
		for _, device := range site.GetDevices() {
			powerKW += device.GetBatteryParameters().GetMaxDischargeKw()
			energyKWh += device.GetBatteryParameters().GetUsableEnergyKwh()
		}
	}
	return powerKW / 1000, energyKWh / 1000
}

func operatingCounts(states []fleet.SiteState, now time.Time, provenance map[string]int) []*gridosv1.OperatingStateDeviceCount {
	values := []gridosv1.FleetOperatingState{
		gridosv1.FleetOperatingState_FLEET_OPERATING_STATE_ON_GRID,
		gridosv1.FleetOperatingState_FLEET_OPERATING_STATE_OFF_GRID_OUTAGE,
		gridosv1.FleetOperatingState_FLEET_OPERATING_STATE_OFF_GRID_NO_HOME_POWER,
		gridosv1.FleetOperatingState_FLEET_OPERATING_STATE_OFF_GRID_OVERCURRENT,
		gridosv1.FleetOperatingState_FLEET_OPERATING_STATE_OFF_GRID_OVERCURRENT_STANDBY,
		gridosv1.FleetOperatingState_FLEET_OPERATING_STATE_TELEMETRY_UNAVAILABLE,
	}
	counts := make(map[gridosv1.FleetOperatingState]uint64)
	for _, state := range states {
		counts[operatingState(state.OperatingState)]++
	}
	result := make([]*gridosv1.OperatingStateDeviceCount, 0, len(values))
	for _, value := range values {
		result = append(result, &gridosv1.OperatingStateDeviceCount{OperatingState: value, Aggregate: deviceCount(counts[value], now, provenance)})
	}
	return result
}

func availabilityCounts(states []fleet.SiteState, now time.Time, provenance map[string]int) []*gridosv1.AvailabilityStateDeviceCount {
	values := []gridosv1.FleetAvailabilityState{
		gridosv1.FleetAvailabilityState_FLEET_AVAILABILITY_STATE_ONLINE,
		gridosv1.FleetAvailabilityState_FLEET_AVAILABILITY_STATE_OFFLINE,
		gridosv1.FleetAvailabilityState_FLEET_AVAILABILITY_STATE_DEGRADED,
		gridosv1.FleetAvailabilityState_FLEET_AVAILABILITY_STATE_STALE,
		gridosv1.FleetAvailabilityState_FLEET_AVAILABILITY_STATE_MAINTENANCE,
	}
	counts := make(map[gridosv1.FleetAvailabilityState]uint64)
	for _, state := range states {
		counts[availabilityState(state.Availability)]++
	}
	result := make([]*gridosv1.AvailabilityStateDeviceCount, 0, len(values))
	for _, value := range values {
		result = append(result, &gridosv1.AvailabilityStateDeviceCount{AvailabilityState: value, Aggregate: deviceCount(counts[value], now, provenance)})
	}
	return result
}

func healthCounts(states []fleet.SiteState, now time.Time, provenance map[string]int) []*gridosv1.HealthStateDeviceCount {
	counts := make(map[gridosv1.FleetHealthState]uint64)
	for _, state := range states {
		switch state.Availability {
		case fleet.Online:
			counts[gridosv1.FleetHealthState_FLEET_HEALTH_STATE_HEALTHY]++
		case fleet.Degraded:
			counts[gridosv1.FleetHealthState_FLEET_HEALTH_STATE_DEGRADED]++
		case fleet.Stale:
			counts[gridosv1.FleetHealthState_FLEET_HEALTH_STATE_UNKNOWN]++
		default:
			counts[gridosv1.FleetHealthState_FLEET_HEALTH_STATE_UNHEALTHY]++
		}
	}
	result := make([]*gridosv1.HealthStateDeviceCount, 0, len(counts))
	for _, value := range []gridosv1.FleetHealthState{
		gridosv1.FleetHealthState_FLEET_HEALTH_STATE_HEALTHY,
		gridosv1.FleetHealthState_FLEET_HEALTH_STATE_DEGRADED,
		gridosv1.FleetHealthState_FLEET_HEALTH_STATE_UNHEALTHY,
		gridosv1.FleetHealthState_FLEET_HEALTH_STATE_UNKNOWN,
	} {
		result = append(result, &gridosv1.HealthStateDeviceCount{HealthState: value, Aggregate: deviceCount(counts[value], now, provenance)})
	}
	return result
}

func deviceCount(count uint64, now time.Time, provenance map[string]int) *gridosv1.FleetDeviceCountAggregate {
	return &gridosv1.FleetDeviceCountAggregate{DeviceCount: count, Metadata: quantity(0, now, 0, provenance).GetMetadata()}
}

func operatingState(state fleet.OperatingState) gridosv1.FleetOperatingState {
	values := map[fleet.OperatingState]gridosv1.FleetOperatingState{
		fleet.OnGrid:                    gridosv1.FleetOperatingState_FLEET_OPERATING_STATE_ON_GRID,
		fleet.OffGridOutage:             gridosv1.FleetOperatingState_FLEET_OPERATING_STATE_OFF_GRID_OUTAGE,
		fleet.OffGridNoHomePower:        gridosv1.FleetOperatingState_FLEET_OPERATING_STATE_OFF_GRID_NO_HOME_POWER,
		fleet.OffGridOvercurrent:        gridosv1.FleetOperatingState_FLEET_OPERATING_STATE_OFF_GRID_OVERCURRENT,
		fleet.OffGridOvercurrentStandby: gridosv1.FleetOperatingState_FLEET_OPERATING_STATE_OFF_GRID_OVERCURRENT_STANDBY,
		fleet.TelemetryUnavailable:      gridosv1.FleetOperatingState_FLEET_OPERATING_STATE_TELEMETRY_UNAVAILABLE,
	}
	return values[state]
}

func availabilityState(state fleet.Availability) gridosv1.FleetAvailabilityState {
	values := map[fleet.Availability]gridosv1.FleetAvailabilityState{
		fleet.Online:      gridosv1.FleetAvailabilityState_FLEET_AVAILABILITY_STATE_ONLINE,
		fleet.Offline:     gridosv1.FleetAvailabilityState_FLEET_AVAILABILITY_STATE_OFFLINE,
		fleet.Degraded:    gridosv1.FleetAvailabilityState_FLEET_AVAILABILITY_STATE_DEGRADED,
		fleet.Stale:       gridosv1.FleetAvailabilityState_FLEET_AVAILABILITY_STATE_STALE,
		fleet.Maintenance: gridosv1.FleetAvailabilityState_FLEET_AVAILABILITY_STATE_MAINTENANCE,
	}
	return values[state]
}

func filterSites(sites []*gridosv1.AuthorizedSite, zones []string) []*gridosv1.AuthorizedSite {
	if len(zones) == 0 {
		return sites
	}
	result := make([]*gridosv1.AuthorizedSite, 0, len(sites))
	for _, site := range sites {
		if slices.Contains(zones, site.GetSite().GetLoadZone()) {
			result = append(result, site)
		}
	}
	return result
}

func aggregateSites(sites []*gridosv1.AuthorizedSite, states []fleet.SiteState, now time.Time) []*gridosv1.SiteLocation {
	groups := make(map[string][]*gridosv1.AuthorizedSite)
	for _, site := range sites {
		groups[site.GetSite().GetH3Cell()] = append(groups[site.GetSite().GetH3Cell()], site)
	}
	cells := geo.Aggregate(sites, states, now)
	result := make([]*gridosv1.SiteLocation, 0, len(cells))
	for _, cell := range cells {
		provenance := siteProvenanceMix(groups[cell.Cell])
		aggregate := &gridosv1.H3SiteAggregate{
			H3Cell: cell.Cell, SiteCount: cell.SiteCount,
			InstalledMw: quantity(cell.InstalledMW, now, 0, provenance), InstalledMwh: quantity(cell.InstalledMWh, now, 0, provenance),
			DispatchableMw: quantity(cell.DispatchableMW, now, cell.Freshness, provenance), ReservedMwh: quantity(cell.ReservedMWh, now, cell.Freshness, provenance),
			OperatingStateCounts:    cellOperatingCounts(cell.OperatingCounts, now, provenance),
			AvailabilityStateCounts: cellAvailabilityCounts(cell.AvailabilityCounts, now, provenance),
			Metadata:                quantity(0, now, cell.Freshness, provenance).GetMetadata(),
		}
		result = append(result, &gridosv1.SiteLocation{Location: &gridosv1.SiteLocation_Aggregate{Aggregate: aggregate}})
	}
	return result
}

func cellOperatingCounts(counts map[fleet.OperatingState]uint64, now time.Time, provenance map[string]int) []*gridosv1.OperatingStateDeviceCount {
	states := make([]fleet.SiteState, 0)
	for state, count := range counts {
		for range count {
			states = append(states, fleet.SiteState{OperatingState: state})
		}
	}
	return operatingCounts(states, now, provenance)
}

func cellAvailabilityCounts(counts map[fleet.Availability]uint64, now time.Time, provenance map[string]int) []*gridosv1.AvailabilityStateDeviceCount {
	states := make([]fleet.SiteState, 0)
	for state, count := range counts {
		for range count {
			states = append(states, fleet.SiteState{Availability: state})
		}
	}
	return availabilityCounts(states, now, provenance)
}

func siteProvenanceMix(sites []*gridosv1.AuthorizedSite) map[string]int {
	mix := make(map[string]int)
	for _, authorized := range sites {
		provenance := authorized.GetSite().GetProvenance().GetProvenance()
		if provenance == gridosv1.DataProvenance_DATA_PROVENANCE_UNSPECIFIED && len(authorized.GetDevices()) > 0 {
			provenance = authorized.GetDevices()[0].GetProvenance().GetProvenance()
		}
		mix[provenanceName(provenance)]++
	}
	return mix
}

func provenanceName(value gridosv1.DataProvenance) string {
	return map[gridosv1.DataProvenance]string{
		gridosv1.DataProvenance_DATA_PROVENANCE_CONFIRMED_PUBLIC:       "confirmed_public",
		gridosv1.DataProvenance_DATA_PROVENANCE_CONFIRMED_SANDBOX:      "confirmed_sandbox",
		gridosv1.DataProvenance_DATA_PROVENANCE_AUTHORIZED_OPERATIONAL: "authorized_operational",
		gridosv1.DataProvenance_DATA_PROVENANCE_DERIVED:                "derived",
		gridosv1.DataProvenance_DATA_PROVENANCE_SIMULATED:              "simulated",
	}[value]
}
