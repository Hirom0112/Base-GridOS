package telemetry

import (
	"context"
	"math"
	"path/filepath"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
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
	assertPhysicalFlow(t, baseline, 1, 0)
	if baseline.GetStateOfEnergyPercent() <= 20 {
		t.Fatalf("baseline physical telemetry = %#v", baseline)
	}
	command := gateway.Command{
		CommandID: "command-physical", IdempotencyKey: "command-physical", DeviceID: "device-physical",
		Generation: 1, SetpointKW: 1, EffectiveAt: now.Add(10 * time.Second), ExpiresAt: now.Add(20 * time.Second),
	}
	acceptPhysicalCommand(t, ctx, store, command, now)
	if err = fleet.Emit(ctx, now.Add(10*time.Second)); err != nil {
		t.Fatal(err)
	}
	active := publisher.batches[1][0]
	assertPhysicalFlow(t, active, -1, 2)
	if active.GetStateOfEnergyPercent() >= baseline.GetStateOfEnergyPercent() {
		t.Fatalf("active physical telemetry = %#v", active)
	}
	if err = fleet.Emit(ctx, now.Add(30*time.Second)); err != nil {
		t.Fatal(err)
	}
	expired := publisher.batches[2][0]
	assertPhysicalFlow(t, expired, 1, 0)
	if math.Abs(expired.GetStateOfEnergyPercent()-active.GetStateOfEnergyPercent()) > 1e-9 {
		t.Fatalf("expired physical telemetry = %#v", expired)
	}
	charge := gateway.Command{
		CommandID: "command-charge", IdempotencyKey: "command-charge", DeviceID: "device-physical",
		Generation: 2, SetpointKW: -3, EffectiveAt: now.Add(40 * time.Second), ExpiresAt: now.Add(50 * time.Second),
	}
	acceptPhysicalCommand(t, ctx, store, charge, now.Add(30*time.Second))
	if err = fleet.Emit(ctx, now.Add(40*time.Second)); err != nil {
		t.Fatal(err)
	}
	charging := publisher.batches[3][0]
	assertPhysicalFlow(t, charging, 3, -2)
	if charging.GetStateOfEnergyPercent() <= expired.GetStateOfEnergyPercent() {
		t.Fatalf("charging physical telemetry = %#v", charging)
	}
}

func acceptPhysicalCommand(t *testing.T, ctx context.Context, store *gateway.Store, command gateway.Command, now time.Time) {
	t.Helper()
	accepted, err := store.AcceptCommand(ctx, command, now)
	if err != nil || !accepted.Accepted {
		t.Fatalf("command acceptance = %#v, %v", accepted, err)
	}
}

func assertPhysicalFlow(t *testing.T, observation *gridosv1.TelemetryObservation, grid, storage float64) {
	t.Helper()
	flow := observation.GetPowerFlow()
	if flow == nil {
		t.Fatalf("missing physical flow: %#v", observation)
	}
	actual := []float64{flow.GetFromGridKw(), flow.GetFromStorageKw(), flow.GetFromSolarKw(), flow.GetNonSolarToHomeKw(), flow.GetToHomeKw()}
	want := []float64{grid, storage, 0, 1, 1}
	for index, value := range actual {
		if value != want[index] {
			t.Fatalf("physical flow = %#v, want %v", flow, want)
		}
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
