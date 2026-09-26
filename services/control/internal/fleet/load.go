package fleet

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type fleetRecord struct {
	DeviceID                 string  `json:"device_id"`
	SiteID                   string  `json:"site_id"`
	WeatherZone              string  `json:"weather_zone"`
	LoadZone                 string  `json:"load_zone"`
	LoadProfileType          string  `json:"load_profile_type"`
	ReliabilityTrait         string  `json:"reliability_trait"`
	County                   *string `json:"county"`
	H3Cell                   string  `json:"h3_cell"`
	HasSolar                 bool    `json:"has_solar"`
	HasAutomaticBackup       bool    `json:"has_automatic_backup"`
	UsableEnergyKWh          float64 `json:"usable_energy_kwh"`
	MaxChargeKW              float64 `json:"max_charge_kw"`
	MaxDischargeKW           float64 `json:"max_discharge_kw"`
	ChargeEfficiency         float64 `json:"charge_efficiency"`
	DischargeEfficiency      float64 `json:"discharge_efficiency"`
	ReservePreferencePercent float64 `json:"reserve_preference_percent"`
	SimulationSeed           int64   `json:"simulation_seed"`
}

type DeviceAsset struct {
	SiteID          string
	UsableEnergyKWh float64
	MaxDischargeKW  float64
	ReservePercent  float64
}

type TelemetryTwin struct {
	twin    *Twin
	devices map[string]DeviceAsset
}

func Load(path string, twin *Twin, now time.Time) ([]*gridosv1.AuthorizedSite, *TelemetryTwin, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	sites := make([]*gridosv1.AuthorizedSite, 0)
	devices := make(map[string]DeviceAsset)
	scanner := bufio.NewScanner(bytes.NewReader(contents))
	for scanner.Scan() {
		var record fleetRecord
		if err = json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return nil, nil, err
		}
		if record.DeviceID == "" || record.SiteID == "" || record.H3Cell == "" || record.UsableEnergyKWh <= 0 || record.MaxDischargeKW < 0 {
			return nil, nil, errors.New("fleet record is incomplete")
		}
		provenance := &gridosv1.Provenance{Provenance: gridosv1.DataProvenance_DATA_PROVENANCE_SIMULATED, SourceId: "fleet-file", ObservedAt: timestamppb.New(now), IngestedAt: timestamppb.New(now), SchemaVersion: "1", SimulationSeed: &record.SimulationSeed}
		site := &gridosv1.Site{SiteId: record.SiteID, WeatherZone: record.WeatherZone, LoadZone: record.LoadZone, H3Cell: record.H3Cell, HasSolar: record.HasSolar, HasAutomaticBackup: record.HasAutomaticBackup, Provenance: provenance, LoadProfileType: record.LoadProfileType, ReliabilityTrait: record.ReliabilityTrait, County: record.County}
		device := &gridosv1.Device{DeviceId: record.DeviceID, SiteId: record.SiteID, BatteryParameters: &gridosv1.BatteryParameters{UsableEnergyKwh: record.UsableEnergyKWh, MaxChargeKw: record.MaxChargeKW, MaxDischargeKw: record.MaxDischargeKW, ChargeEfficiency: record.ChargeEfficiency, DischargeEfficiency: record.DischargeEfficiency}, LastSeenAt: timestamppb.New(now), Provenance: provenance}
		sites = append(sites, &gridosv1.AuthorizedSite{Site: site, Devices: []*gridosv1.Device{device}})
		devices[record.DeviceID] = DeviceAsset{SiteID: record.SiteID, UsableEnergyKWh: record.UsableEnergyKWh, MaxDischargeKW: record.MaxDischargeKW, ReservePercent: record.ReservePreferencePercent}
		twin.Accept(SiteState{SiteID: record.SiteID, OperatingState: TelemetryUnavailable, Availability: Offline, Provenance: "simulated"})
	}
	if err = scanner.Err(); err != nil {
		return nil, nil, err
	}
	return sites, &TelemetryTwin{twin: twin, devices: devices}, nil
}

func (adapter *TelemetryTwin) Accept(observation *gridosv1.TelemetryObservation) {
	asset, found := adapter.devices[observation.GetDeviceId()]
	if !found || observation.GetObservationTime() == nil {
		return
	}
	state := telemetryState(observation)
	energyKWh := asset.UsableEnergyKWh * observation.GetStateOfEnergyPercent() / 100
	availableKWh := max(energyKWh-asset.UsableEnergyKWh*asset.ReservePercent/100, 0)
	dispatchableKW := min(asset.MaxDischargeKW, availableKWh)
	if state.OperatingState != OnGrid || state.Availability == Stale {
		dispatchableKW = 0
		availableKWh = 0
	}
	state.SiteID = asset.SiteID
	state.ObservedAt = observation.GetObservationTime().AsTime()
	state.DispatchableKW = dispatchableKW
	state.DispatchableKWh = availableKWh
	state.EnergyKWh = energyKWh
	state.ReserveKWh = asset.UsableEnergyKWh * asset.ReservePercent / 100
	state.Provenance = "simulated"
	adapter.twin.Accept(state)
}

func telemetryState(observation *gridosv1.TelemetryObservation) SiteState {
	state := SiteState{Availability: Online}
	if observation.GetValueState() == gridosv1.ValueState_VALUE_STATE_STALE {
		state.Availability = Stale
	}
	switch operating := observation.GetOperatingState().(type) {
	case *gridosv1.TelemetryObservation_OnGrid:
		state.OperatingState = OnGrid
		state.BackupHoursCurrent = operating.OnGrid.GetEstimatedBackupHoursAtCurrentUsage()
		state.BackupHours750W = operating.OnGrid.GetEstimatedBackupHoursAt_750Watts()
	case *gridosv1.TelemetryObservation_OffGridOutage:
		state.OperatingState = OffGridOutage
	case *gridosv1.TelemetryObservation_OffGridNoHomePower:
		state.OperatingState = OffGridNoHomePower
	case *gridosv1.TelemetryObservation_OffGridOvercurrent:
		state.OperatingState = OffGridOvercurrent
	case *gridosv1.TelemetryObservation_OffGridOvercurrentStandby:
		state.OperatingState = OffGridOvercurrentStandby
	default:
		state.OperatingState = TelemetryUnavailable
		state.Availability = Offline
	}
	return state
}
