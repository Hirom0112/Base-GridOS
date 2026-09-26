package api

import (
	"slices"
	"sort"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet"
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

func operatingCounts(states []fleet.SiteState, now time.Time) []*gridosv1.OperatingStateDeviceCount {
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
		result = append(result, &gridosv1.OperatingStateDeviceCount{OperatingState: value, Aggregate: deviceCount(counts[value], now)})
	}
	return result
}

func availabilityCounts(states []fleet.SiteState, now time.Time) []*gridosv1.AvailabilityStateDeviceCount {
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
		result = append(result, &gridosv1.AvailabilityStateDeviceCount{AvailabilityState: value, Aggregate: deviceCount(counts[value], now)})
	}
	return result
}

func healthCounts(states []fleet.SiteState, now time.Time) []*gridosv1.HealthStateDeviceCount {
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
		result = append(result, &gridosv1.HealthStateDeviceCount{HealthState: value, Aggregate: deviceCount(counts[value], now)})
	}
	return result
}

func deviceCount(count uint64, now time.Time) *gridosv1.FleetDeviceCountAggregate {
	return &gridosv1.FleetDeviceCountAggregate{DeviceCount: count, Metadata: quantity(0, now, 0, nil).GetMetadata()}
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

func aggregateSites(sites []*gridosv1.AuthorizedSite, now time.Time) []*gridosv1.SiteLocation {
	groups := make(map[string][]*gridosv1.AuthorizedSite)
	for _, site := range sites {
		groups[site.GetSite().GetH3Cell()] = append(groups[site.GetSite().GetH3Cell()], site)
	}
	cells := make([]string, 0, len(groups))
	for cell := range groups {
		cells = append(cells, cell)
	}
	sort.Strings(cells)
	result := make([]*gridosv1.SiteLocation, 0, len(cells))
	for _, cell := range cells {
		powerMW, energyMWh := installedCapacity(groups[cell])
		aggregate := &gridosv1.H3SiteAggregate{H3Cell: cell, SiteCount: uint64(len(groups[cell])), InstalledMw: quantity(powerMW, now, 0, nil), InstalledMwh: quantity(energyMWh, now, 0, nil)}
		result = append(result, &gridosv1.SiteLocation{Location: &gridosv1.SiteLocation_Aggregate{Aggregate: aggregate}})
	}
	return result
}
