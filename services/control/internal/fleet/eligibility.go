package fleet

import "time"

type ExclusionReason string

const (
	ExclusionNone                      ExclusionReason = "NONE"
	ExcludedOffline                    ExclusionReason = "OFFLINE"
	ExcludedStale                      ExclusionReason = "STALE"
	ExcludedOffGrid                    ExclusionReason = "ISLANDED_OR_OFF_GRID"
	ExcludedOvercurrent                ExclusionReason = "OVERCURRENT"
	ExcludedMaintenanceLock            ExclusionReason = "MAINTENANCE_LOCK"
	ExcludedUnderReserve               ExclusionReason = "UNDER_RESERVE"
	ExcludedAlarm                      ExclusionReason = "ALARM"
	ExcludedOutsideRegion              ExclusionReason = "OUTSIDE_REGION"
	ExcludedOutsideParticipationWindow ExclusionReason = "OUTSIDE_PARTICIPATION_WINDOW"
)

type DeviceEligibility struct {
	Online             bool
	Fresh              bool
	OperatingState     OperatingState
	MaintenanceLocked  bool
	StateOfEnergy      float64
	EffectiveReserve   float64
	HasAlarm           bool
	InRegion           bool
	ParticipationStart time.Time
	ParticipationEnd   time.Time
}

type EligibilityResult struct {
	Eligible bool
	Reason   ExclusionReason
}

func EvaluateEligibility(device DeviceEligibility, at time.Time) EligibilityResult {
	reason := exclusionReason(device, at)
	return EligibilityResult{Eligible: reason == ExclusionNone, Reason: reason}
}

func exclusionReason(device DeviceEligibility, at time.Time) ExclusionReason {
	if !device.Online {
		return ExcludedOffline
	}
	if !device.Fresh {
		return ExcludedStale
	}
	if device.OperatingState == OffGridOvercurrent || device.OperatingState == OffGridOvercurrentStandby {
		return ExcludedOvercurrent
	}
	if device.OperatingState != OnGrid {
		return ExcludedOffGrid
	}
	if device.MaintenanceLocked {
		return ExcludedMaintenanceLock
	}
	if device.StateOfEnergy < device.EffectiveReserve {
		return ExcludedUnderReserve
	}
	if device.HasAlarm {
		return ExcludedAlarm
	}
	if !device.InRegion {
		return ExcludedOutsideRegion
	}
	if at.Before(device.ParticipationStart) || at.After(device.ParticipationEnd) {
		return ExcludedOutsideParticipationWindow
	}
	return ExclusionNone
}
