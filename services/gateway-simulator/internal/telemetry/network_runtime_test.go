package telemetry

import (
	"context"
	"errors"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
	"github.com/Hirom0112/Base-GridOS/services/gateway-simulator/internal/battery"
)

func TestNetworkFailureBuffersAndReplaysOnNextCadence(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store := openStore(t, ctx)
	start := time.Date(2026, 8, 12, 18, 0, 0, 0, time.UTC)
	publisher := &recoveringBatchPublisher{failFirst: true, recovered: make(chan struct{}, 1)}
	fleet, err := NewFleet(store, []Device{testPhysicalDevice("device-1")}, Profiles{"home": {}}, 20*time.Millisecond, publisher)
	if err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() { result <- fleet.Run(ctx, start) }()
	select {
	case <-publisher.recovered:
	case <-time.After(time.Second):
		t.Fatal("publisher did not recover")
	}
	time.Sleep(10 * time.Millisecond)
	cancel()
	err = <-result
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("run error=%v", err)
	}
	if len(publisher.batches) < 2 {
		t.Fatalf("publish attempts=%d", len(publisher.batches))
	}
	replayed := publisher.batches[1]
	if len(replayed) != 2 {
		t.Fatalf("replayed observations=%d", len(replayed))
	}
	if !replayed[0].GetSourceTime().AsTime().Equal(start) || !replayed[1].GetSourceTime().AsTime().Equal(start.Add(20*time.Millisecond)) {
		t.Fatalf("source times=%s,%s", replayed[0].GetSourceTime().AsTime(), replayed[1].GetSourceTime().AsTime())
	}
	buffered, err := store.BufferedObservations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(buffered) != 0 {
		t.Fatalf("buffered observations=%d", len(buffered))
	}
}

func TestRuntimeEffectsApplyOnlyToSelectedDevice(t *testing.T) {
	ctx := context.Background()
	store := openStore(t, ctx)
	now := time.Date(2026, 8, 12, 18, 0, 0, 0, time.UTC)
	publisher := &recoveringBatchPublisher{}
	fleet, err := NewFleet(store, []Device{testPhysicalDevice("selected"), testPhysicalDevice("healthy")}, Profiles{"home": {}}, time.Second, publisher)
	if err != nil {
		t.Fatal(err)
	}
	fleet.SetEffects(fixedEffects{kind: "DROPPED_MESSAGES", deviceID: "selected"})
	if err := fleet.Emit(ctx, now); err != nil {
		t.Fatal(err)
	}
	if len(publisher.batches) != 1 || len(publisher.batches[0]) != 1 || publisher.batches[0][0].GetDeviceId() != "healthy" {
		t.Fatalf("published=%v", publisher.batches)
	}
}

type recoveringBatchPublisher struct {
	batches   [][]*gridosv1.TelemetryObservation
	failFirst bool
	recovered chan struct{}
}

func (publisher *recoveringBatchPublisher) Publish(context.Context, *gridosv1.TelemetryObservation) error {
	return errors.New("single publish is not expected")
}

func (publisher *recoveringBatchPublisher) PublishBatch(_ context.Context, observations []*gridosv1.TelemetryObservation) error {
	publisher.batches = append(publisher.batches, append([]*gridosv1.TelemetryObservation(nil), observations...))
	if publisher.failFirst && len(publisher.batches) == 1 {
		return ErrPublishUnavailable
	}
	if publisher.recovered != nil {
		select {
		case publisher.recovered <- struct{}{}:
		default:
		}
	}
	return nil
}

type fixedEffects struct {
	kind     string
	deviceID string
}

func testPhysicalDevice(id string) Device {
	return Device{DeviceID: id, LoadProfileType: "home", SimulationSeed: 1, Parameters: battery.Parameters{
		UsableEnergyKWh: 10, MaxChargeKW: 2, MaxDischargeKW: 2, ChargeEfficiency: 1, DischargeEfficiency: 1,
	}}
}

func (fixedEffects) Advance(time.Time) {
}

func (effects fixedEffects) Affects(kind, deviceID string) bool {
	return effects.kind == kind && effects.deviceID == deviceID
}
