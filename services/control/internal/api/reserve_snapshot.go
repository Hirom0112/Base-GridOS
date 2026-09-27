package api

import (
	"errors"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/fleet/policy"
	"github.com/Hirom0112/Base-GridOS/services/control/internal/safety"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func appendFrozenSite(optimization *gridosv1.OptimizationRequest, canonical *safety.CanonicalState, site *gridosv1.AuthorizedSite, state fleet.SiteState, reserve *policy.ReserveState, at time.Time) error {
	if reserve != nil && reserve.TravelFlex != nil {
		binding, err := frozenTravelFlexBinding(site.GetSite().GetSiteId(), reserve.TravelFlex, at)
		if err != nil {
			return err
		}
		optimization.EligibilitySnapshot.TravelFlexBindings = append(optimization.EligibilitySnapshot.TravelFlexBindings, binding)
	}
	optimization.Sites = append(optimization.Sites, &gridosv1.ForecastSite{
		SiteId: site.GetSite().GetSiteId(), LoadProfileType: site.GetSite().GetLoadProfileType(),
		LoadZone: site.GetSite().GetLoadZone(), WeatherZone: site.GetSite().GetWeatherZone(), County: site.GetSite().GetCounty(),
	})
	for _, device := range site.GetDevices() {
		parameters := device.GetBatteryParameters()
		energy := state.EnergyKWh
		available := state.OperatingState == fleet.OnGrid && state.Availability == fleet.Online
		baseReserve := max(state.ReserveKWh, state.HardwareFloorKWh)
		var baseField, flexField *float64
		if reserve != nil {
			baseReserve = max(baseReserve, parameters.GetUsableEnergyKwh()*reserve.BasePercent/100)
			baseField = &baseReserve
			if reserve.TravelFlexPercent != nil {
				flex := min(baseReserve, max(state.HardwareFloorKWh, parameters.GetUsableEnergyKwh()*(*reserve.TravelFlexPercent)/100))
				flexField = &flex
			}
		}
		basis, err := frozenReserveBasis(device.GetDeviceId(), state, parameters.GetUsableEnergyKwh(), baseReserve, reserve, at)
		if err != nil {
			return err
		}
		optimization.EligibilitySnapshot.ReserveBases = append(optimization.EligibilitySnapshot.ReserveBases, basis)
		optimization.Devices = append(optimization.Devices, &gridosv1.DeviceState{
			DeviceId: device.GetDeviceId(), UsableEnergyKwh: parameters.GetUsableEnergyKwh(), EnergyKwh: energy,
			HardwareFloorKwh: state.HardwareFloorKWh, EffectiveReserveKwh: baseReserve, BaseReserveKwh: baseField, TravelFlexReserveKwh: flexField,
			MaxChargeKw: parameters.GetMaxChargeKw(), MaxDischargeKw: parameters.GetMaxDischargeKw(),
			ChargeEfficiency: parameters.GetChargeEfficiency(), DischargeEfficiency: parameters.GetDischargeEfficiency(),
			AvailabilityProbability: boolFloat(available), Stale: state.Availability == fleet.Stale, TelemetryObservedAt: timestamppb.New(state.ObservedAt), LoadZone: site.GetSite().GetLoadZone(),
			SiteId: site.GetSite().GetSiteId(), ReliabilityTrait: site.GetSite().GetReliabilityTrait(),
		})
		if available {
			optimization.EligibilitySnapshot.EligibleDeviceIds = append(optimization.EligibilitySnapshot.EligibleDeviceIds, device.GetDeviceId())
		}
		observedAt := state.ObservedAt
		canonical.Devices[device.GetDeviceId()] = safety.DeviceState{
			EnergyKWh: &energy, UsableCapacityKWh: parameters.GetUsableEnergyKwh(), HardwareReserveKWh: state.HardwareFloorKWh, PlanReserveKWh: baseReserve, TravelFlexReserveKWh: flexField,
			MaxChargeKW: parameters.GetMaxChargeKw(), MaxDischargeKW: parameters.GetMaxDischargeKw(), ChargeEfficiency: parameters.GetChargeEfficiency(), DischargeEfficiency: parameters.GetDischargeEfficiency(),
			Available: available, MaintenanceLocked: state.Availability == fleet.Maintenance, TelemetryAt: &observedAt, FreshnessLimit: 30 * time.Second, MeterExportLimitKW: parameters.GetMaxDischargeKw(), InterconnectionLimitKW: parameters.GetMaxDischargeKw(),
		}
	}
	return nil
}

func frozenReserveBasis(deviceID string, state fleet.SiteState, usable, effective float64, reserve *policy.ReserveState, at time.Time) (*gridosv1.FrozenReserveBasis, error) {
	basis := &gridosv1.FrozenReserveBasis{DeviceId: deviceID, HardwareFloorKwh: state.HardwareFloorKWh,
		PlanReserveKwh: state.ReserveKWh, PolicyFloorKwh: state.HardwareFloorKWh,
		EffectiveReserveKwh: effective, PolicyVersion: "fleet-file",
		Provenance: gridosv1.DataProvenance_DATA_PROVENANCE_SIMULATED, IssuedAt: timestamppb.New(at)}
	if reserve == nil {
		return basis, nil
	}
	basis.PlanReserveKwh = usable * reserve.PlanPercent / 100
	basis.PolicyFloorKwh = usable * reserve.PolicyFloorPercent / 100
	basis.PolicyVersion = reserve.PolicyVersion
	if reserve.OverridePercent == nil {
		return basis, nil
	}
	floor := usable * *reserve.OverridePercent / 100
	basis.OverrideFloorKwh = &floor
	basis.OverrideSourceId = reserve.OverrideSourceID
	basis.OverridePolicyVersion = reserve.OverridePolicyVersion
	switch reserve.OverrideReason {
	case policy.OverrideWeather:
		basis.OverrideReason = gridosv1.ReserveOverrideReason_RESERVE_OVERRIDE_REASON_WEATHER
	case policy.OverrideOutageRisk:
		basis.OverrideReason = gridosv1.ReserveOverrideReason_RESERVE_OVERRIDE_REASON_OUTAGE_RISK
	case policy.OverrideHealth:
		basis.OverrideReason = gridosv1.ReserveOverrideReason_RESERVE_OVERRIDE_REASON_HEALTH
	case policy.OverrideStaleTelemetry:
		basis.OverrideReason = gridosv1.ReserveOverrideReason_RESERVE_OVERRIDE_REASON_STALE_TELEMETRY
	case policy.OverrideAlarm:
		basis.OverrideReason = gridosv1.ReserveOverrideReason_RESERVE_OVERRIDE_REASON_ALARM
	case policy.OverrideCommunications:
		basis.OverrideReason = gridosv1.ReserveOverrideReason_RESERVE_OVERRIDE_REASON_COMMUNICATIONS
	case "EARLY_RETURN":
		basis.OverrideReason = gridosv1.ReserveOverrideReason_RESERVE_OVERRIDE_REASON_EARLY_RETURN
	default:
		return nil, errors.New("unknown frozen reserve override reason")
	}
	return basis, nil
}

func frozenTravelFlexBinding(siteID string, window *policy.TravelFlexBinding, at time.Time) (*gridosv1.FrozenTravelFlexBinding, error) {
	if window == nil {
		return nil, nil
	}
	binding := &gridosv1.FrozenTravelFlexBinding{WindowId: window.WindowID, MemberId: window.MemberID,
		SiteId: siteID, StartTime: timestamppb.New(window.Start), EndTime: timestamppb.New(window.End),
		CreditCents: window.CreditCents, ConsentVersion: window.ConsentVersion, PolicyVersion: window.PolicyVersion,
		Provenance: gridosv1.DataProvenance_DATA_PROVENANCE_SIMULATED, IssuedAt: timestamppb.New(at)}
	switch window.CreditType {
	case policy.FixedDaily:
		binding.CreditType = gridosv1.TravelFlexCreditType_TRAVEL_FLEX_CREDIT_TYPE_FIXED_DAILY
	case policy.FixedEvent:
		binding.CreditType = gridosv1.TravelFlexCreditType_TRAVEL_FLEX_CREDIT_TYPE_FIXED_EVENT
	case policy.FixedAnnual:
		binding.CreditType = gridosv1.TravelFlexCreditType_TRAVEL_FLEX_CREDIT_TYPE_FIXED_ANNUAL
	default:
		return nil, errors.New("unknown frozen travel flex credit type")
	}
	return binding, nil
}
