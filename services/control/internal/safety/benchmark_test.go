package safety

import (
	"strconv"
	"testing"
	"time"
)

func BenchmarkValidate5000(b *testing.B) {
	plan, state := benchmarkInputs(5000, 24)
	b.ResetTimer()
	for range b.N {
		approval, violations := Validate(plan, state)
		if !approval.Approved {
			b.Fatalf("unexpected rejection: %#v", violations)
		}
	}
}

func BenchmarkValidate5000x288(b *testing.B) {
	plan, state := benchmarkInputs(5000, 288)
	b.ResetTimer()
	for range b.N {
		approval, violations := Validate(plan, state)
		if !approval.Approved {
			b.Fatalf("unexpected rejection: %d violations, first=%#v", len(violations), violations[0])
		}
	}
}

func benchmarkInputs(deviceCount, intervals int) (Plan, CanonicalState) {
	now := time.Date(2026, 9, 26, 18, 0, 0, 0, time.UTC)
	duration := 5 * time.Minute
	charge := make([]float64, intervals)
	discharge := make([]float64, intervals)
	export := make([]float64, intervals)
	energy := make([]float64, intervals+1)
	energy[0] = 30
	for interval := range intervals {
		discharge[interval] = 2
		export[interval] = 1
		energy[interval+1] = energy[interval] - 2.0/0.95/12
	}
	plan := Plan{Interval: duration, Boundary: MeterNetExport, PolicyVersion: "benchmark", Generation: 1, EffectiveAt: now.Add(time.Minute), ExpiresAt: now.Add(time.Minute).Add(time.Duration(intervals) * duration), TargetKW: float64(deviceCount)}
	states := make(map[string]DeviceState, deviceCount)
	for index := range deviceCount {
		deviceID := "device-" + strconv.Itoa(index)
		storedEnergy := 30.0
		telemetryAt := now.Add(-time.Minute)
		plan.Devices = append(plan.Devices, DevicePlan{DeviceID: deviceID, ChargeKW: charge, DischargeKW: discharge, EnergyKWh: energy, MeterExportKW: export})
		states[deviceID] = DeviceState{EnergyKWh: &storedEnergy, UsableCapacityKWh: 40, HardwareReserveKWh: 5, PlanReserveKWh: 10, MaxChargeKW: 10, MaxDischargeKW: 10, ChargeEfficiency: 0.95, DischargeEfficiency: 0.95, Available: true, TelemetryAt: &telemetryAt, FreshnessLimit: 5 * time.Minute, MeterExportLimitKW: 10, InterconnectionLimitKW: 10}
	}
	state := CanonicalState{Now: now, Boundary: MeterNetExport, PolicyVersion: "benchmark", ExpectedGeneration: 1, Devices: states}
	return plan, state
}
