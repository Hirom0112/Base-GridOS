package safety

import (
	"testing"
	"time"
)

func TestReserveSelectionRequiresConsentedValueAndHardwareFloor(t *testing.T) {
	plan, canonical := validInputs()
	energy := 9.0
	flex := 6.0
	state := canonical.Devices["device-1"]
	state.EnergyKWh = &energy
	state.TravelFlexReserveKWh = &flex
	canonical.Devices["device-1"] = state
	plan.Interval = time.Hour
	plan.ExpiresAt = plan.EffectiveAt.Add(time.Hour)
	plan.TargetKW = 1.9
	plan.Devices[0].DischargeKW[0] = 1.9
	plan.Devices[0].MeterExportKW[0] = 1.9
	plan.Devices[0].EnergyKWh = []float64{9, 7}
	plan.Devices[0].ReserveSelection = "BASE"
	plan.Devices[0].SelectedReserveKWh = 8
	_, violations := Validate(plan, canonical)
	if !violationCodes(violations)[EnergyBelowReserve] {
		t.Fatalf("base selection did not enforce base reserve: %+v", violations)
	}
	plan.Devices[0].ReserveSelection = "TRAVEL_FLEX"
	plan.Devices[0].SelectedReserveKWh = 6
	approval, violations := Validate(plan, canonical)
	if !approval.Approved || len(violations) != 0 {
		t.Fatalf("consented flex selection rejected: %+v", violations)
	}
	state.TravelFlexReserveKWh = nil
	canonical.Devices["device-1"] = state
	_, violations = Validate(plan, canonical)
	if !violationCodes(violations)[ReserveSelectionMismatch] {
		t.Fatalf("unconsented flex selection passed: %+v", violations)
	}
	belowHardware := 4.0
	state.TravelFlexReserveKWh = &belowHardware
	canonical.Devices["device-1"] = state
	_, violations = Validate(plan, canonical)
	if !violationCodes(violations)[ReserveSelectionMismatch] {
		t.Fatalf("below-hardware flex selection passed: %+v", violations)
	}
}
