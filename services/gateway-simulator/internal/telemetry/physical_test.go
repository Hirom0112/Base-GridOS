package telemetry

import (
	"context"
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/battery"
	"github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/gateway"
)

func TestPhysicalFleetTracksCommandAndHomeLoad(t *testing.T) {
	ctx := context.Background()
	store := openStore(t, ctx)
	now := time.Date(2026, 8, 12, 18, 0, 0, 0, time.UTC)
	profile := [7][96]float64{}
	for day := range profile {
		for slot := range profile[day] {
			profile[day][slot] = 0.25
		}
	}
	publisher := &recoveringBatchPublisher{}
	device := Device{
		DeviceID: "device-physical", LoadProfileType: "home", SimulationSeed: 42,
		Parameters: battery.Parameters{
			UsableEnergyKWh: 10, HardwareFloorKWh: 2, MaxChargeKW: 2, MaxDischargeKW: 2,
			ChargeEfficiency: 1, DischargeEfficiency: 1,
		},
	}
	fleet, err := NewFleet(store, []Device{device}, Profiles{"home": profile}, 10*time.Second, publisher)
	if err != nil {
		t.Fatal(err)
	}
	if err = fleet.Emit(ctx, now); err != nil {
		t.Fatal(err)
	}
	baseline := publisher.batches[0][0]
	if baseline.GetStateOfEnergyPercent() <= 20 || baseline.GetPowerFlow() == nil || baseline.GetPowerFlow().GetFromGridKw() != 1 || baseline.GetPowerFlow().GetFromStorageKw() != 0 || baseline.GetPowerFlow().GetFromSolarKw() != 0 || baseline.GetPowerFlow().GetNonSolarToHomeKw() != 1 || baseline.GetPowerFlow().GetToHomeKw() != 1 {
		t.Fatalf("baseline physical telemetry = %#v", baseline)
	}
	command := gateway.Command{
		CommandID: "command-physical", IdempotencyKey: "command-physical", DeviceID: "device-physical",
		Generation: 1, SetpointKW: 1, EffectiveAt: now.Add(10 * time.Second), ExpiresAt: now.Add(20 * time.Second),
	}
	accepted, err := store.AcceptCommand(ctx, command, now)
	if err != nil || !accepted.Accepted {
		t.Fatalf("command acceptance = %#v, %v", accepted, err)
	}
	if err = fleet.Emit(ctx, now.Add(10*time.Second)); err != nil {
		t.Fatal(err)
	}
	active := publisher.batches[1][0]
	if active.GetPowerFlow().GetFromStorageKw() != 2 || active.GetPowerFlow().GetFromGridKw() != -1 || active.GetStateOfEnergyPercent() >= baseline.GetStateOfEnergyPercent() {
		t.Fatalf("active physical telemetry = %#v", active)
	}
	if err = fleet.Emit(ctx, now.Add(30*time.Second)); err != nil {
		t.Fatal(err)
	}
	expired := publisher.batches[2][0]
	if expired.GetPowerFlow().GetFromStorageKw() != 0 || expired.GetPowerFlow().GetFromGridKw() != 1 || math.Abs(expired.GetStateOfEnergyPercent()-active.GetStateOfEnergyPercent()) > 1e-9 {
		t.Fatalf("expired physical telemetry = %#v", expired)
	}
}

func TestPhysicalProfileUsesSourceTimeInCentralWeek(t *testing.T) {
	profiles, err := ReadProfiles(filepath.Join("..", "..", "..", "..", "testdata", "fixtures", "public", "load-profiles", "residential-week.csv"))
	if err != nil {
		t.Fatal(err)
	}
	central, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Fatal(err)
	}
	first := time.Date(2026, 1, 7, 0, 0, 0, 0, central)
	load, err := profiles.LoadKW("RESHIDG_COAST", first)
	if err != nil || math.Abs(load-1.284) > 1e-9 {
		t.Fatalf("Wednesday midnight load = %v, %v", load, err)
	}
	if _, err = profiles.LoadKW("unknown", first); err == nil {
		t.Fatal("unknown profile accepted")
	}
}
