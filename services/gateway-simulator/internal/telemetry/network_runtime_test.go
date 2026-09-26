package telemetry

import (
	"context"
	"errors"
	"testing"
	"time"

	gridosv1 "github.com/Hirom0112/Base-GridOS/contracts/gen/go/gridos/v1"
)

func TestNetworkFailureBuffersAndReplaysOnNextCadence(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store := openStore(t, ctx)
	start := time.Date(2026, 8, 12, 18, 0, 0, 0, time.UTC)
	publisher := &recoveringBatchPublisher{cancel: cancel}
	fleet, err := NewFleet(store, []string{"device-1"}, time.Millisecond, publisher)
	if err != nil {
		t.Fatal(err)
	}
	err = fleet.Run(ctx, start)
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
	if !replayed[0].GetSourceTime().AsTime().Equal(start) || !replayed[1].GetSourceTime().AsTime().Equal(start.Add(time.Millisecond)) {
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

type recoveringBatchPublisher struct {
	batches [][]*gridosv1.TelemetryObservation
	cancel  context.CancelFunc
}

func (publisher *recoveringBatchPublisher) Publish(context.Context, *gridosv1.TelemetryObservation) error {
	return errors.New("single publish is not expected")
}

func (publisher *recoveringBatchPublisher) PublishBatch(_ context.Context, observations []*gridosv1.TelemetryObservation) error {
	publisher.batches = append(publisher.batches, append([]*gridosv1.TelemetryObservation(nil), observations...))
	if len(publisher.batches) == 1 {
		return ErrPublishUnavailable
	}
	publisher.cancel()
	return nil
}
