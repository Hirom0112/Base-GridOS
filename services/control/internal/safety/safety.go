package safety

import (
	"crypto/sha256"
	"encoding/json"
	"math"
	"time"

	"github.com/Hirom0112/Base-GridOS/services/control/internal/observability"
)

type MeasurementBoundary string

const (
	MeterNetExport  MeasurementBoundary = "METER_NET_EXPORT"
	BatteryTerminal MeasurementBoundary = "BATTERY_TERMINAL"
)

type ViolationCode string

const (
	NonFiniteValue              ViolationCode = "NON_FINITE_VALUE"
	WrongVectorLength           ViolationCode = "WRONG_VECTOR_LENGTH"
	ChargeBound                 ViolationCode = "CHARGE_BOUND"
	DischargeBound              ViolationCode = "DISCHARGE_BOUND"
	SimultaneousChargeDischarge ViolationCode = "SIMULTANEOUS_CHARGE_DISCHARGE"
	EnergyBalanceDrift          ViolationCode = "ENERGY_BALANCE_DRIFT"
	EnergyBelowReserve          ViolationCode = "ENERGY_BELOW_RESERVE"
	Unavailable                 ViolationCode = "UNAVAILABLE"
	MaintenanceLock             ViolationCode = "MAINTENANCE_LOCK"
	StaleTelemetry              ViolationCode = "STALE_TELEMETRY"
	MeterExportLimit            ViolationCode = "METER_EXPORT_LIMIT"
	InterconnectionLimit        ViolationCode = "INTERCONNECTION_LIMIT"
	WrongMeasurementBoundary    ViolationCode = "WRONG_MEASUREMENT_BOUNDARY"
	WrongGeneration             ViolationCode = "WRONG_GENERATION"
	EffectiveTimeInPast         ViolationCode = "EFFECTIVE_TIME_IN_PAST"
	ExpiryBeforeEffective       ViolationCode = "EXPIRY_BEFORE_EFFECTIVE"
	PolicyVersionMismatch       ViolationCode = "POLICY_VERSION_MISMATCH"
	UndeclaredShortfall         ViolationCode = "UNDECLARED_SHORTFALL"
	RampRate                    ViolationCode = "RAMP_RATE"
	ReapprovalRequired          ViolationCode = "REAPPROVAL_REQUIRED"
	MissingStateOfCharge        ViolationCode = "MISSING_STATE_OF_CHARGE"
	MissingFreshness            ViolationCode = "MISSING_FRESHNESS"
	ContradictoryInput          ViolationCode = "CONTRADICTORY_INPUT"
	ReserveSelectionMismatch    ViolationCode = "RESERVE_SELECTION_MISMATCH"
)

type ReserveSelection string

const (
	BaseReserveSelection       ReserveSelection = "BASE"
	TravelFlexReserveSelection ReserveSelection = "TRAVEL_FLEX"
)

type Plan struct {
	Interval          time.Duration
	Boundary          MeasurementBoundary
	PolicyVersion     string
	Generation        int64
	EffectiveAt       time.Time
	ExpiresAt         time.Time
	TargetKW          float64
	DeclaredShortfall float64
	Devices           []DevicePlan
}

type DevicePlan struct {
	DeviceID           string
	ReserveSelection   ReserveSelection
	SelectedReserveKWh float64
	ChargeKW           []float64
	DischargeKW        []float64
	EnergyKWh          []float64
	MeterExportKW      []float64
}

type TravelFlexWindow struct {
	Start      time.Time
	End        time.Time
	ReserveKWh float64
	ReturnedAt *time.Time
}

type DeviceState struct {
	EnergyKWh              *float64
	UsableCapacityKWh      float64
	HardwareReserveKWh     float64
	PlanReserveKWh         float64
	DynamicReserveKWh      float64
	TravelFlex             *TravelFlexWindow
	TravelFlexReserveKWh   *float64
	MaxChargeKW            float64
	MaxDischargeKW         float64
	ChargeEfficiency       float64
	DischargeEfficiency    float64
	Available              bool
	MaintenanceLocked      bool
	TelemetryAt            *time.Time
	FreshnessLimit         time.Duration
	MeterExportLimitKW     float64
	InterconnectionLimitKW float64
	MaxRampKWPerMinute     float64
	PreviousMeterExportKW  float64
}

type CanonicalState struct {
	Now                time.Time
	Boundary           MeasurementBoundary
	PolicyVersion      string
	ExpectedGeneration int64
	Devices            map[string]DeviceState
}

type Approval struct {
	Approved    bool
	InputDigest [32]byte
}

type Violation struct {
	Code     ViolationCode
	DeviceID string
	Interval int
}

const comparisonTolerance = 1e-9

func Validate(plan Plan, canonical CanonicalState) (Approval, []Violation) {
	violations := validateEnvelope(plan, canonical)
	intervals := intervalCount(plan)
	actualByInterval := make([]float64, intervals)
	for _, devicePlan := range plan.Devices {
		state, ok := canonical.Devices[devicePlan.DeviceID]
		if !ok {
			violations = append(violations, Violation{Code: ContradictoryInput, DeviceID: devicePlan.DeviceID})
			continue
		}
		deviceViolations, actual := validateDevice(plan, devicePlan, state, canonical.Now, intervals)
		violations = append(violations, deviceViolations...)
		for interval := range actual {
			actualByInterval[interval] += actual[interval]
		}
	}
	for interval, actual := range actualByInterval {
		shortfall := math.Max(0, plan.TargetKW-actual)
		if shortfall-plan.DeclaredShortfall > comparisonTolerance {
			violations = append(violations, Violation{Code: UndeclaredShortfall, Interval: interval})
		}
	}
	if len(violations) != 0 {
		observability.ProcessMetrics.RecordSafetyRejection()
		return Approval{}, violations
	}
	canonical.Now = time.Time{}
	encoded, err := json.Marshal(struct {
		Plan      Plan
		Canonical CanonicalState
	}{plan, canonical})
	if err != nil {
		observability.ProcessMetrics.RecordSafetyRejection()
		return Approval{}, []Violation{{Code: ContradictoryInput}}
	}
	return Approval{Approved: true, InputDigest: sha256.Sum256(encoded)}, nil
}

func RevalidateApproval(plan Plan, canonical CanonicalState, approved Approval) (Approval, []Violation) {
	current, violations := Validate(plan, canonical)
	if !current.Approved {
		return current, violations
	}
	if !approved.Approved || approved.InputDigest != current.InputDigest {
		return Approval{}, []Violation{{Code: ReapprovalRequired}}
	}
	return current, nil
}

func validateEnvelope(plan Plan, canonical CanonicalState) []Violation {
	violations := make([]Violation, 0)
	if plan.Boundary != canonical.Boundary {
		violations = append(violations, Violation{Code: WrongMeasurementBoundary})
	}
	if plan.Generation != canonical.ExpectedGeneration {
		violations = append(violations, Violation{Code: WrongGeneration})
	}
	if plan.EffectiveAt.Before(canonical.Now) {
		violations = append(violations, Violation{Code: EffectiveTimeInPast})
	}
	if !plan.ExpiresAt.After(plan.EffectiveAt) {
		violations = append(violations, Violation{Code: ExpiryBeforeEffective})
	}
	if plan.PolicyVersion != canonical.PolicyVersion {
		violations = append(violations, Violation{Code: PolicyVersionMismatch})
	}
	if plan.Interval <= 0 || !finite(plan.TargetKW, plan.DeclaredShortfall) || plan.TargetKW < 0 || plan.DeclaredShortfall < 0 {
		violations = append(violations, Violation{Code: ContradictoryInput})
	}
	return violations
}

func validateDevice(plan Plan, proposed DevicePlan, state DeviceState, now time.Time, intervals int) ([]Violation, []float64) {
	violations := validateDeviceState(proposed.DeviceID, state, now)
	actual := make([]float64, intervals)
	if len(proposed.ChargeKW) != intervals || len(proposed.DischargeKW) != intervals || len(proposed.MeterExportKW) != intervals || len(proposed.EnergyKWh) != intervals+1 {
		return append(violations, Violation{Code: WrongVectorLength, DeviceID: proposed.DeviceID}), actual
	}
	if state.EnergyKWh == nil || invalidPhysics(state) {
		return violations, actual
	}
	energy := *state.EnergyKWh
	if math.Abs(proposed.EnergyKWh[0]-energy) > comparisonTolerance {
		violations = append(violations, Violation{Code: EnergyBalanceDrift, DeviceID: proposed.DeviceID})
	}
	reserve, selected := selectedReserve(proposed, state, now)
	if !selected {
		violations = append(violations, Violation{Code: ReserveSelectionMismatch, DeviceID: proposed.DeviceID})
		return violations, actual
	}
	previousExport := state.PreviousMeterExportKW
	for interval := 0; interval < intervals; interval++ {
		stepViolations, next := validateInterval(plan.Interval, proposed, state, energy, reserve, interval)
		violations = append(violations, stepViolations...)
		if state.MaxRampKWPerMinute > 0 && math.Abs(proposed.MeterExportKW[interval]-previousExport) > state.MaxRampKWPerMinute*plan.Interval.Minutes()+comparisonTolerance {
			violations = append(violations, Violation{Code: RampRate, DeviceID: proposed.DeviceID, Interval: interval})
		}
		previousExport = proposed.MeterExportKW[interval]
		energy = next
		actual[interval] = proposed.MeterExportKW[interval]
	}
	return violations, actual
}

func selectedReserve(proposed DevicePlan, state DeviceState, now time.Time) (float64, bool) {
	if proposed.ReserveSelection == "" {
		return EffectiveReserve(state, now), true
	}
	reserve := math.Max(state.HardwareReserveKWh, math.Max(state.PlanReserveKWh, state.DynamicReserveKWh))
	switch proposed.ReserveSelection {
	case BaseReserveSelection:
	case TravelFlexReserveSelection:
		flex := state.TravelFlexReserveKWh
		if flex == nil || !finite(*flex) || *flex < state.HardwareReserveKWh || *flex > state.PlanReserveKWh {
			return 0, false
		}
		reserve = math.Max(state.HardwareReserveKWh, math.Max(*flex, state.DynamicReserveKWh))
	default:
		return 0, false
	}
	return reserve, finite(proposed.SelectedReserveKWh) && math.Abs(proposed.SelectedReserveKWh-reserve) <= comparisonTolerance
}

func validateDeviceState(deviceID string, state DeviceState, now time.Time) []Violation {
	violations := make([]Violation, 0)
	if state.EnergyKWh == nil {
		violations = append(violations, Violation{Code: MissingStateOfCharge, DeviceID: deviceID})
	}
	if state.TelemetryAt == nil || state.FreshnessLimit <= 0 {
		violations = append(violations, Violation{Code: MissingFreshness, DeviceID: deviceID})
	} else if now.Sub(*state.TelemetryAt) > state.FreshnessLimit {
		violations = append(violations, Violation{Code: StaleTelemetry, DeviceID: deviceID})
	}
	if !state.Available {
		violations = append(violations, Violation{Code: Unavailable, DeviceID: deviceID})
	}
	if state.MaintenanceLocked {
		violations = append(violations, Violation{Code: MaintenanceLock, DeviceID: deviceID})
	}
	if invalidPhysics(state) {
		violations = append(violations, Violation{Code: ContradictoryInput, DeviceID: deviceID})
	}
	return violations
}

func validateInterval(duration time.Duration, proposed DevicePlan, state DeviceState, energy, reserve float64, interval int) ([]Violation, float64) {
	violations := make([]Violation, 0)
	charge := proposed.ChargeKW[interval]
	discharge := proposed.DischargeKW[interval]
	export := proposed.MeterExportKW[interval]
	claimedEnergy := proposed.EnergyKWh[interval+1]
	if !finite(charge, discharge, export, claimedEnergy) {
		return append(violations, Violation{Code: NonFiniteValue, DeviceID: proposed.DeviceID, Interval: interval}), energy
	}
	if charge < 0 || charge-state.MaxChargeKW > comparisonTolerance {
		violations = append(violations, Violation{Code: ChargeBound, DeviceID: proposed.DeviceID, Interval: interval})
	}
	if discharge < 0 || discharge-state.MaxDischargeKW > comparisonTolerance {
		violations = append(violations, Violation{Code: DischargeBound, DeviceID: proposed.DeviceID, Interval: interval})
	}
	if charge > 0 && discharge > 0 {
		violations = append(violations, Violation{Code: SimultaneousChargeDischarge, DeviceID: proposed.DeviceID, Interval: interval})
	}
	hours := duration.Hours()
	next := energy + state.ChargeEfficiency*charge*hours - discharge*hours/state.DischargeEfficiency
	if math.Abs(next-claimedEnergy) > comparisonTolerance {
		violations = append(violations, Violation{Code: EnergyBalanceDrift, DeviceID: proposed.DeviceID, Interval: interval})
	}
	if energy+comparisonTolerance < reserve || next+comparisonTolerance < reserve {
		violations = append(violations, Violation{Code: EnergyBelowReserve, DeviceID: proposed.DeviceID, Interval: interval})
	}
	if export-state.MeterExportLimitKW > comparisonTolerance {
		violations = append(violations, Violation{Code: MeterExportLimit, DeviceID: proposed.DeviceID, Interval: interval})
	}
	if export-state.InterconnectionLimitKW > comparisonTolerance {
		violations = append(violations, Violation{Code: InterconnectionLimit, DeviceID: proposed.DeviceID, Interval: interval})
	}
	return violations, next
}

func intervalCount(plan Plan) int {
	if plan.Interval <= 0 || !plan.ExpiresAt.After(plan.EffectiveAt) {
		return 0
	}
	return int(plan.ExpiresAt.Sub(plan.EffectiveAt) / plan.Interval)
}

func invalidPhysics(state DeviceState) bool {
	values := []float64{state.UsableCapacityKWh, state.HardwareReserveKWh, state.PlanReserveKWh, state.DynamicReserveKWh, state.MaxChargeKW, state.MaxDischargeKW, state.ChargeEfficiency, state.DischargeEfficiency, state.MeterExportLimitKW, state.InterconnectionLimitKW, state.MaxRampKWPerMinute, state.PreviousMeterExportKW}
	if !finite(values...) || invalidNonnegativeBounds(state) {
		return true
	}
	if state.EnergyKWh != nil && (!finite(*state.EnergyKWh) || *state.EnergyKWh < 0 || *state.EnergyKWh > state.UsableCapacityKWh) {
		return true
	}
	return state.ChargeEfficiency <= 0 || state.ChargeEfficiency > 1 || state.DischargeEfficiency <= 0 || state.DischargeEfficiency > 1 || state.HardwareReserveKWh > state.UsableCapacityKWh || state.PlanReserveKWh > state.UsableCapacityKWh || state.DynamicReserveKWh > state.UsableCapacityKWh
}

func invalidNonnegativeBounds(state DeviceState) bool {
	return state.UsableCapacityKWh <= 0 || state.HardwareReserveKWh < 0 || state.PlanReserveKWh < 0 || state.DynamicReserveKWh < 0 || state.MaxChargeKW < 0 || state.MaxDischargeKW < 0 || state.MeterExportLimitKW < 0 || state.InterconnectionLimitKW < 0 || state.MaxRampKWPerMinute < 0
}

func EffectiveReserve(state DeviceState, now time.Time) float64 {
	planReserve := state.PlanReserveKWh
	travel := state.TravelFlex
	if travel != nil && state.DynamicReserveKWh == 0 && !now.Before(travel.Start) && now.Before(travel.End) && (travel.ReturnedAt == nil || now.Before(*travel.ReturnedAt)) {
		planReserve = travel.ReserveKWh
	}
	return math.Max(state.HardwareReserveKWh, math.Max(planReserve, state.DynamicReserveKWh))
}

func finite(values ...float64) bool {
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
	}
	return true
}
