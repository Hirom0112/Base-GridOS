package safety

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type goldenFixture struct {
	CanonicalState struct {
		Devices []goldenDevice `json:"devices"`
	} `json:"canonical_state"`
	ExpectedPlan struct {
		Schedules  []goldenSchedule `json:"schedules"`
		Shortfalls []struct {
			ShortfallKW float64 `json:"shortfall_kw"`
		} `json:"shortfalls"`
	} `json:"expected_dispatch_plan"`
	ExpectedValidation struct {
		Approved          bool     `json:"approved"`
		ViolationFamilies []string `json:"violation_families"`
	} `json:"expected_validation"`
	Request struct {
		Intervals []struct {
			DurationHours float64 `json:"duration_hours"`
			TargetKW      float64 `json:"target_kw"`
		} `json:"intervals"`
	} `json:"optimization_request"`
}

type goldenDevice struct {
	Available              bool    `json:"available"`
	DeviceID               string  `json:"device_id"`
	DischargeEfficiency    float64 `json:"discharge_efficiency"`
	DynamicOverridePercent float64 `json:"dynamic_override_percent"`
	EnergyKWh              float64 `json:"energy_kwh"`
	HardwareFloorPercent   float64 `json:"hardware_floor_percent"`
	Maintenance            bool    `json:"maintenance"`
	MaxDischargeKW         float64 `json:"max_discharge_kw"`
	ReservePercent         float64 `json:"reserve_percent"`
	Stale                  bool    `json:"stale"`
	UsableEnergyKWh        float64 `json:"usable_energy_kwh"`
}

type goldenSchedule struct {
	DeviceID  string `json:"device_id"`
	Intervals []struct {
		DischargeKW       float64 `json:"discharge_kw"`
		ExpectedEnergyKWh float64 `json:"expected_energy_kwh"`
		GridServiceKW     float64 `json:"grid_service_kw"`
	} `json:"intervals"`
}

func TestGoldenPythonAndGoSafetyAgree(t *testing.T) {
	paths, err := filepath.Glob("../../../../testdata/golden/plans/*.json")
	if err != nil || len(paths) == 0 {
		t.Fatalf("golden fixture discovery: %v, paths=%d", err, len(paths))
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			fixture := readGolden(t, path)
			plan, state := goldenInputs(fixture)
			approval, violations := Validate(plan, state)
			if approval.Approved != fixture.ExpectedValidation.Approved {
				t.Fatalf("Python approved=%v, Go approved=%v, violations=%#v", fixture.ExpectedValidation.Approved, approval.Approved, violations)
			}
			if !approval.Approved {
				assertGoldenFamilies(t, fixture.ExpectedValidation.ViolationFamilies, violations)
			}
		})
	}
}

func readGolden(t *testing.T, path string) goldenFixture {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var fixture goldenFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func goldenInputs(fixture goldenFixture) (Plan, CanonicalState) {
	now := time.Date(2026, 9, 26, 18, 0, 0, 0, time.UTC)
	duration := time.Duration(fixture.Request.Intervals[0].DurationHours * float64(time.Hour))
	plan := Plan{Interval: duration, Boundary: MeterNetExport, PolicyVersion: "golden", Generation: 1, EffectiveAt: now.Add(time.Minute), ExpiresAt: now.Add(time.Minute).Add(duration), TargetKW: fixture.Request.Intervals[0].TargetKW}
	if len(fixture.ExpectedPlan.Shortfalls) > 0 {
		plan.DeclaredShortfall = fixture.ExpectedPlan.Shortfalls[0].ShortfallKW
	}
	states := make(map[string]DeviceState, len(fixture.CanonicalState.Devices))
	for _, device := range fixture.CanonicalState.Devices {
		states[device.DeviceID] = goldenState(device, now)
	}
	for _, schedule := range fixture.ExpectedPlan.Schedules {
		device := states[schedule.DeviceID]
		plan.Devices = append(plan.Devices, goldenDevicePlan(schedule, *device.EnergyKWh))
	}
	return plan, CanonicalState{Now: now, Boundary: MeterNetExport, PolicyVersion: "golden", ExpectedGeneration: 1, Devices: states}
}

func goldenState(device goldenDevice, now time.Time) DeviceState {
	energy := device.EnergyKWh
	telemetryAt := now.Add(-time.Minute)
	if device.Stale {
		telemetryAt = now.Add(-time.Hour)
	}
	return DeviceState{EnergyKWh: &energy, UsableCapacityKWh: device.UsableEnergyKWh, HardwareReserveKWh: device.UsableEnergyKWh * device.HardwareFloorPercent / 100, PlanReserveKWh: device.UsableEnergyKWh * device.ReservePercent / 100, DynamicReserveKWh: device.UsableEnergyKWh * device.DynamicOverridePercent / 100, MaxChargeKW: device.MaxDischargeKW, MaxDischargeKW: device.MaxDischargeKW, ChargeEfficiency: device.DischargeEfficiency, DischargeEfficiency: device.DischargeEfficiency, Available: device.Available, MaintenanceLocked: device.Maintenance, TelemetryAt: &telemetryAt, FreshnessLimit: 5 * time.Minute, MeterExportLimitKW: device.MaxDischargeKW, InterconnectionLimitKW: device.MaxDischargeKW}
}

func goldenDevicePlan(schedule goldenSchedule, initialEnergy float64) DevicePlan {
	plan := DevicePlan{DeviceID: schedule.DeviceID, EnergyKWh: []float64{initialEnergy}}
	for _, interval := range schedule.Intervals {
		plan.ChargeKW = append(plan.ChargeKW, 0)
		plan.DischargeKW = append(plan.DischargeKW, interval.DischargeKW)
		plan.EnergyKWh = append(plan.EnergyKWh, interval.ExpectedEnergyKWh)
		plan.MeterExportKW = append(plan.MeterExportKW, interval.GridServiceKW)
	}
	return plan
}

func assertGoldenFamilies(t *testing.T, expected []string, violations []Violation) {
	t.Helper()
	actual := make(map[string]bool, len(violations))
	for _, violation := range violations {
		actual[string(violation.Code)] = true
	}
	for _, family := range expected {
		if !actual[family] {
			t.Fatalf("expected violation family %s, got %#v", family, violations)
		}
	}
}
