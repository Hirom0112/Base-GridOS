package telemetry

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
)

func TestAnchorSkipsMissedCadenceSlots(t *testing.T) {
	wallStart := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	logicalStart := time.Date(2026, 8, 12, 18, 0, 0, 0, time.UTC)
	cadence := 15 * time.Second
	sourceStep := 5 * time.Minute
	first := sourceAt(logicalStart, wallStart, wallStart, cadence, sourceStep)
	if !first.Equal(logicalStart) {
		t.Fatalf("first source time = %s", first)
	}
	late := sourceAt(logicalStart, wallStart, wallStart.Add(46*time.Second), cadence, sourceStep)
	if !late.Equal(logicalStart.Add(3 * sourceStep)) {
		t.Fatalf("late source time = %s", late)
	}
	if !late.After(first.Add(sourceStep)) {
		t.Fatalf("missed cadence slots were replayed: %s", late)
	}
	epochStart := wallStart.Add(7 * time.Second).Truncate(cadence)
	wallNow := epochStart.Add(46 * time.Second)
	live := sourceAt(epochStart, epochStart, wallNow, cadence, cadence)
	if !live.Equal(wallNow.Truncate(cadence)) {
		t.Fatalf("live source time = %s", live)
	}
}

func TestAnchorRecordsOneGapBeforeCurrentPhysicalSample(t *testing.T) {
	ctx := context.Background()
	store := openStore(t, ctx)
	start := time.Date(2026, 8, 12, 18, 0, 0, 0, time.UTC)
	publisher := &recoveringBatchPublisher{}
	fleet, err := NewFleet(store, []Device{testPhysicalDevice("device-1")}, Profiles{"home": {}}, time.Minute, publisher)
	if err != nil {
		t.Fatal(err)
	}
	if err := fleet.Emit(ctx, start); err != nil {
		t.Fatal(err)
	}
	current := start.Add(4 * time.Minute)
	if err := fleet.emitSkipped(ctx, current.Add(-time.Minute), current); err != nil {
		t.Fatal(err)
	}
	latest := publisher.batches[1]
	if len(latest) != 2 {
		t.Fatalf("gap and current observations = %d", len(latest))
	}
	if latest[0].GetValueState() != gridosv1.ValueState_VALUE_STATE_MISSING || !latest[0].GetSourceTime().AsTime().Equal(current.Add(-time.Minute)) {
		t.Fatalf("last skipped slot gap = %#v", latest[0])
	}
	if latest[1].GetValueState() != gridosv1.ValueState_VALUE_STATE_PRESENT || !latest[1].GetSourceTime().AsTime().Equal(current) || latest[1].GetPowerFlow() == nil {
		t.Fatalf("current physical sample = %#v", latest[1])
	}
}

func TestRepeatedSlotPublishesOnceWithoutError(t *testing.T) {
	ctx := context.Background()
	store := openStore(t, ctx)
	publisher := &recoveringBatchPublisher{}
	fleet, err := NewFleet(store, []Device{testPhysicalDevice("device-repeat")}, Profiles{"home": {}}, time.Minute, publisher)
	if err != nil {
		t.Fatal(err)
	}
	slot := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	if err := fleet.Emit(ctx, slot); err != nil {
		t.Fatal(err)
	}
	if err := fleet.Emit(ctx, slot); err != nil {
		t.Fatalf("repeated slot stopped scheduler: %v", err)
	}
	if len(publisher.batches) != 1 {
		t.Fatalf("published batches = %d, want 1", len(publisher.batches))
	}
}

func TestRepeatedSlotKeepsFleetRunningAfterDeviceRejection(t *testing.T) {
	ctx := context.Background()
	store := openStore(t, ctx)
	publisher := &recoveringBatchPublisher{}
	devices := []Device{testPhysicalDevice("device-good"), testPhysicalDevice("device-bad")}
	devices[1].LoadProfileType = "bad"
	fleet, err := NewFleet(store, devices, Profiles{"home": {}, "bad": {}}, time.Minute, publisher)
	if err != nil {
		t.Fatal(err)
	}
	delete(fleet.profiles, "bad")
	var logs bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previous) })
	slot := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	if err := fleet.Emit(ctx, slot); err != nil {
		t.Fatalf("one device rejection stopped fleet: %v", err)
	}
	if !strings.Contains(logs.String(), "telemetry producer rejections=1") || len(publisher.batches) != 1 || len(publisher.batches[0]) != 1 {
		t.Fatalf("logs=%q batches=%d", logs.String(), len(publisher.batches))
	}
}
