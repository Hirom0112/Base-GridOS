package battery

import (
	"math"
	"testing"
	"time"
)

func closeTo(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.00005 {
		t.Fatalf("got %.4f want %.4f", got, want)
	}
}

func TestWorkedExample(t *testing.T) {
	result, err := Estimate(Parameters{
		UsableEnergyKWh:     39.2,
		ReservePercent:      40,
		MaxDischargeKW:      10,
		DischargeEfficiency: 0.95,
	}, 74, 3.1, 2*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	closeTo(t, result.StoredKWh, 29.0080)
	closeTo(t, result.ReserveKWh, 15.6800)
	closeTo(t, result.AboveReserveKWh, 13.3280)
	closeTo(t, result.ACAvailableKWh, 12.6616)
	closeTo(t, result.DischargeKW, 6.3308)
	closeTo(t, result.NetExportKW, 3.2308)
	closeTo(t, result.BackupHoursCurrentUsage, 4.0844)
	closeTo(t, result.BackupHours750W, 16.8821)
}

func TestStepEnergyAndLimits(t *testing.T) {
	model, err := New(Parameters{
		UsableEnergyKWh:     20,
		HardwareFloorKWh:    2,
		MaxChargeKW:         5,
		MaxDischargeKW:      6,
		ChargeEfficiency:    0.9,
		DischargeEfficiency: 0.8,
		RampLimitKWPerHour:  4,
		OvercurrentLimitKW:  8,
	}, 10, OnGrid)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := model.Step(Input{ChargeKW: 1, DischargeKW: 1, Duration: time.Hour}); err == nil {
		t.Fatal("simultaneous charge and discharge accepted")
	}
	if _, err := model.Step(Input{ChargeKW: 6, Duration: time.Hour}); err == nil {
		t.Fatal("charge above power bound accepted")
	}
	if _, err := model.Step(Input{DischargeKW: 5, Duration: time.Hour}); err == nil {
		t.Fatal("ramp violation accepted")
	}
	charged, err := model.Step(Input{ChargeKW: 4, Duration: time.Hour, CurrentLoadKW: 2})
	if err != nil {
		t.Fatal(err)
	}
	closeTo(t, charged.EnergyKWh, 13.6)
	if _, err := model.Step(Input{DischargeKW: 4, Duration: 3 * time.Hour}); err == nil {
		t.Fatal("energy below reserve accepted")
	}
}

func TestTemperatureDerateAndOperatingStates(t *testing.T) {
	model, err := New(Parameters{
		UsableEnergyKWh:     20,
		HardwareFloorKWh:    2,
		MaxChargeKW:         10,
		MaxDischargeKW:      10,
		ChargeEfficiency:    1,
		DischargeEfficiency: 1,
		RampLimitKWPerHour:  20,
		OvercurrentLimitKW:  5,
		TemperatureDerate: func(celsius float64) float64 {
			if celsius >= 40 {
				return 0.5
			}
			return 1
		},
	}, 15, OnGrid)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := model.Step(Input{DischargeKW: 6, Duration: time.Hour, TemperatureCelsius: 40}); err == nil {
		t.Fatal("temperature derate not enforced")
	}
	outage, err := model.Step(Input{Duration: time.Minute, GridOutage: true, CurrentLoadKW: 2})
	if err != nil {
		t.Fatal(err)
	}
	if outage.OperatingState != OffGridOutage || outage.GridServiceCapacityKW != 0 {
		t.Fatalf("outage state=%s capacity=%v", outage.OperatingState, outage.GridServiceCapacityKW)
	}
	noPower, err := model.Step(Input{Duration: time.Minute, HomePowerUnavailable: true})
	if err != nil {
		t.Fatal(err)
	}
	if noPower.OperatingState != OffGridNoHomePower {
		t.Fatalf("state=%s", noPower.OperatingState)
	}
	model.state = OffGridOutage
	overcurrent, err := model.Step(Input{Duration: time.Minute, CurrentLoadKW: 6})
	if err != nil {
		t.Fatal(err)
	}
	if overcurrent.OperatingState != OffGridOvercurrent {
		t.Fatalf("state=%s", overcurrent.OperatingState)
	}
	standby, err := model.Step(Input{Duration: time.Minute, ProtectionStandby: true})
	if err != nil {
		t.Fatal(err)
	}
	if standby.OperatingState != OffGridOvercurrentStandby {
		t.Fatalf("state=%s", standby.OperatingState)
	}
	restored, err := model.Step(Input{Duration: time.Minute, GridConnectionVerified: true})
	if err != nil {
		t.Fatal(err)
	}
	if restored.OperatingState != OnGrid || restored.GridServiceCapacityKW <= 0 {
		t.Fatalf("restored state=%s capacity=%v", restored.OperatingState, restored.GridServiceCapacityKW)
	}
}
