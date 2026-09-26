package safety

import (
	"testing"
)

func TestAggregateCommitment(t *testing.T) {
	plan, state := validInputs()
	plan.TargetKW = 3
	plan.DeclaredShortfall = 1
	approval, violations := Validate(plan, state)
	if !approval.Approved {
		t.Fatalf("declared shortfall should be valid: %#v", violations)
	}
	plan.DeclaredShortfall = 0
	approval, violations = Validate(plan, state)
	if approval.Approved || !violationCodes(violations)[UndeclaredShortfall] {
		t.Fatalf("unmet commitment should be rejected: %#v", violations)
	}
}

func TestRampRate(t *testing.T) {
	plan, state := validInputs()
	plan.ExpiresAt = plan.EffectiveAt.Add(2 * plan.Interval)
	plan.Devices[0].ChargeKW = []float64{0, 0}
	plan.Devices[0].DischargeKW = []float64{2, 4}
	plan.Devices[0].MeterExportKW = []float64{2, 4}
	plan.Devices[0].EnergyKWh = []float64{20, 20 - 2.0/0.95/12, 20 - 6.0/0.95/12}
	device := state.Devices["device-1"]
	device.MaxRampKWPerMinute = 1
	device.PreviousMeterExportKW = 2
	state.Devices["device-1"] = device
	approval, violations := Validate(plan, state)
	if !approval.Approved {
		t.Fatalf("feasible ramp should pass: %#v", violations)
	}
	device.MaxRampKWPerMinute = 0.1
	state.Devices["device-1"] = device
	approval, violations = Validate(plan, state)
	if approval.Approved || !violationCodes(violations)[RampRate] {
		t.Fatalf("excessive ramp should be rejected: %#v", violations)
	}
	plan.Devices[0].MeterExportKW[0] = 5
	plan.Devices[0].MeterExportKW[1] = 5
	device.MaxRampKWPerMinute = 0.5
	state.Devices["device-1"] = device
	_, violations = Validate(plan, state)
	if !violationCodes(violations)[RampRate] {
		t.Fatalf("first interval must respect prior setpoint: %#v", violations)
	}
}
