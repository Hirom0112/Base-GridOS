package safety

import (
	"math"
	"testing"
	"time"
)

func validInputs() (Plan, CanonicalState) {
	now := time.Date(2026, 9, 26, 18, 0, 0, 0, time.UTC)
	energy := 20.0
	freshness := now.Add(-time.Minute)
	plan := Plan{
		Interval:          5 * time.Minute,
		Boundary:          MeterNetExport,
		PolicyVersion:     "policy-1",
		Generation:        4,
		EffectiveAt:       now.Add(time.Minute),
		ExpiresAt:         now.Add(6 * time.Minute),
		TargetKW:          2,
		DeclaredShortfall: 0,
		Devices: []DevicePlan{{
			DeviceID:      "device-1",
			ChargeKW:      []float64{0},
			DischargeKW:   []float64{2},
			EnergyKWh:     []float64{20, 20 - 2.0/0.95/12},
			MeterExportKW: []float64{2},
		}},
	}
	state := CanonicalState{
		Now:                now,
		Boundary:           MeterNetExport,
		PolicyVersion:      "policy-1",
		ExpectedGeneration: 4,
		Devices: map[string]DeviceState{
			"device-1": {
				EnergyKWh:              &energy,
				UsableCapacityKWh:      30,
				HardwareReserveKWh:     5,
				PlanReserveKWh:         8,
				DynamicReserveKWh:      0,
				MaxChargeKW:            10,
				MaxDischargeKW:         10,
				ChargeEfficiency:       0.95,
				DischargeEfficiency:    0.95,
				Available:              true,
				MaintenanceLocked:      false,
				TelemetryAt:            &freshness,
				FreshnessLimit:         5 * time.Minute,
				MeterExportLimitKW:     10,
				InterconnectionLimitKW: 10,
			},
		},
	}
	return plan, state
}

func violationCodes(violations []Violation) map[ViolationCode]bool {
	codes := make(map[ViolationCode]bool, len(violations))
	for _, violation := range violations {
		codes[violation.Code] = true
	}
	return codes
}

func TestValidateChecksEverySafetyCondition(t *testing.T) {
	tests := []struct {
		name   string
		code   ViolationCode
		mutate func(*Plan, *CanonicalState)
	}{
		{"non-finite value", NonFiniteValue, func(plan *Plan, _ *CanonicalState) { plan.Devices[0].DischargeKW[0] = math.NaN() }},
		{"wrong vector length", WrongVectorLength, func(plan *Plan, _ *CanonicalState) { plan.Devices[0].ChargeKW = nil }},
		{"charge bound", ChargeBound, func(plan *Plan, _ *CanonicalState) { plan.Devices[0].ChargeKW[0] = 11 }},
		{"discharge bound", DischargeBound, func(plan *Plan, _ *CanonicalState) { plan.Devices[0].DischargeKW[0] = 11 }},
		{"simultaneous charge and discharge", SimultaneousChargeDischarge, func(plan *Plan, _ *CanonicalState) { plan.Devices[0].ChargeKW[0] = 1 }},
		{"energy balance drift", EnergyBalanceDrift, func(plan *Plan, _ *CanonicalState) { plan.Devices[0].EnergyKWh[1] += 0.01 }},
		{"energy below reserve", EnergyBelowReserve, func(_ *Plan, state *CanonicalState) {
			state.Devices["device-1"] = withEnergy(state.Devices["device-1"], 8.1)
		}},
		{"availability false", Unavailable, func(_ *Plan, state *CanonicalState) {
			device := state.Devices["device-1"]
			device.Available = false
			state.Devices["device-1"] = device
		}},
		{"maintenance lock", MaintenanceLock, func(_ *Plan, state *CanonicalState) {
			device := state.Devices["device-1"]
			device.MaintenanceLocked = true
			state.Devices["device-1"] = device
		}},
		{"stale telemetry", StaleTelemetry, func(_ *Plan, state *CanonicalState) {
			timestamp := state.Now.Add(-6 * time.Minute)
			device := state.Devices["device-1"]
			device.TelemetryAt = &timestamp
			state.Devices["device-1"] = device
		}},
		{"meter export limit", MeterExportLimit, func(plan *Plan, state *CanonicalState) {
			plan.Devices[0].MeterExportKW[0] = 3
			device := state.Devices["device-1"]
			device.MeterExportLimitKW = 2
			state.Devices["device-1"] = device
		}},
		{"interconnection limit", InterconnectionLimit, func(plan *Plan, state *CanonicalState) {
			plan.Devices[0].MeterExportKW[0] = 3
			device := state.Devices["device-1"]
			device.InterconnectionLimitKW = 2
			state.Devices["device-1"] = device
		}},
		{"wrong measurement boundary", WrongMeasurementBoundary, func(plan *Plan, _ *CanonicalState) { plan.Boundary = BatteryTerminal }},
		{"wrong generation", WrongGeneration, func(plan *Plan, _ *CanonicalState) { plan.Generation++ }},
		{"effective time in past", EffectiveTimeInPast, func(plan *Plan, state *CanonicalState) { plan.EffectiveAt = state.Now.Add(-time.Second) }},
		{"expiry before effective", ExpiryBeforeEffective, func(plan *Plan, _ *CanonicalState) { plan.ExpiresAt = plan.EffectiveAt.Add(-time.Second) }},
		{"policy version mismatch", PolicyVersionMismatch, func(plan *Plan, _ *CanonicalState) { plan.PolicyVersion = "policy-2" }},
		{"undeclared shortfall", UndeclaredShortfall, func(plan *Plan, _ *CanonicalState) { plan.TargetKW = 3 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan, state := validInputs()
			test.mutate(&plan, &state)
			_, violations := Validate(plan, state)
			if !violationCodes(violations)[test.code] {
				t.Fatalf("expected %s, got %#v", test.code, violations)
			}
		})
	}
}

func TestValidateApprovesValidPlan(t *testing.T) {
	plan, state := validInputs()
	approval, violations := Validate(plan, state)
	if !approval.Approved || len(violations) != 0 {
		t.Fatalf("expected approval, got %#v and %#v", approval, violations)
	}
}

func TestReserveHardwareFloorSurvivesZeroPlan(t *testing.T) {
	plan, state := validInputs()
	device := state.Devices["device-1"]
	device.PlanReserveKWh = 0
	if reserve := EffectiveReserve(device, state.Now); reserve != 5 {
		t.Fatalf("expected hardware floor 5, got %v", reserve)
	}
	device = withEnergy(device, 5.1)
	state.Devices["device-1"] = device
	plan.Devices[0].EnergyKWh = []float64{5.1, 5.1 - 2.0/0.95/12}
	_, violations := Validate(plan, state)
	if !violationCodes(violations)[EnergyBelowReserve] {
		t.Fatalf("expected hardware reserve rejection, got %#v", violations)
	}
}

func TestReserveWeatherOverrideRaisesFloor(t *testing.T) {
	plan, state := validInputs()
	device := state.Devices["device-1"]
	device.DynamicReserveKWh = 10
	device = withEnergy(device, 10.1)
	state.Devices["device-1"] = device
	plan.Devices[0].EnergyKWh = []float64{10.1, 10.1 - 2.0/0.95/12}
	_, violations := Validate(plan, state)
	if !violationCodes(violations)[EnergyBelowReserve] {
		t.Fatalf("expected dynamic reserve rejection, got %#v", violations)
	}
}

func TestReserveInactiveTravelFlexCannotLowerFloor(t *testing.T) {
	tests := []struct {
		name   string
		window func(time.Time) TravelFlexWindow
	}{
		{"before start", func(now time.Time) TravelFlexWindow {
			return TravelFlexWindow{Start: now.Add(time.Hour), End: now.Add(2 * time.Hour)}
		}},
		{"after end", func(now time.Time) TravelFlexWindow {
			return TravelFlexWindow{Start: now.Add(-2 * time.Hour), End: now.Add(-time.Hour)}
		}},
		{"after early return", func(now time.Time) TravelFlexWindow {
			returned := now.Add(-time.Minute)
			return TravelFlexWindow{Start: now.Add(-time.Hour), End: now.Add(time.Hour), ReturnedAt: &returned}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan, state := validInputs()
			device := state.Devices["device-1"]
			window := test.window(state.Now)
			device.TravelFlex = &window
			device = withEnergy(device, 8.1)
			state.Devices["device-1"] = device
			plan.Devices[0].EnergyKWh = []float64{8.1, 8.1 - 2.0/0.95/12}
			_, violations := Validate(plan, state)
			if !violationCodes(violations)[EnergyBelowReserve] {
				t.Fatalf("expected plan reserve rejection, got %#v", violations)
			}
		})
	}
}

func TestReserveOverrideBlocksTravelFlex(t *testing.T) {
	plan, state := validInputs()
	device := state.Devices["device-1"]
	device.DynamicReserveKWh = 10
	device.TravelFlex = &TravelFlexWindow{Start: state.Now.Add(-time.Hour), End: state.Now.Add(time.Hour)}
	device = withEnergy(device, 10.1)
	state.Devices["device-1"] = device
	plan.Devices[0].EnergyKWh = []float64{10.1, 10.1 - 2.0/0.95/12}
	_, violations := Validate(plan, state)
	if !violationCodes(violations)[EnergyBelowReserve] {
		t.Fatalf("expected override reserve rejection, got %#v", violations)
	}
}

func TestFailClosedOnMissingOrContradictoryState(t *testing.T) {
	tests := []struct {
		name   string
		code   ViolationCode
		mutate func(*Plan, *CanonicalState)
	}{
		{"missing state of charge", MissingStateOfCharge, func(_ *Plan, state *CanonicalState) {
			device := state.Devices["device-1"]
			device.EnergyKWh = nil
			state.Devices["device-1"] = device
		}},
		{"missing freshness", MissingFreshness, func(_ *Plan, state *CanonicalState) {
			device := state.Devices["device-1"]
			device.TelemetryAt = nil
			state.Devices["device-1"] = device
		}},
		{"energy exceeds capacity", ContradictoryInput, func(plan *Plan, state *CanonicalState) {
			device := withEnergy(state.Devices["device-1"], 31)
			state.Devices["device-1"] = device
			plan.Devices[0].EnergyKWh = []float64{31, 31 - 2.0/0.95/12}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan, state := validInputs()
			test.mutate(&plan, &state)
			approval, violations := Validate(plan, state)
			if approval.Approved || !violationCodes(violations)[test.code] {
				t.Fatalf("expected rejection with %s, got %#v and %#v", test.code, approval, violations)
			}
		})
	}
}

func withEnergy(device DeviceState, energy float64) DeviceState {
	device.EnergyKWh = &energy
	return device
}
