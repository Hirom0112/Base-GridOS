package gateway

import (
	"context"
	"path/filepath"
	"testing"
)

func TestBatchConfirmationIsAtomic(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, id := range []string{"first", "second"} {
		if err := store.BufferObservation(ctx, id, []byte(id)); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.ConfirmObservations(ctx, []string{"first", "missing"}); err == nil {
		t.Fatal("missing observation accepted")
	}
	buffered, err := store.BufferedObservations(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(buffered) != 2 {
		t.Fatalf("failed batch removed observations: %d", len(buffered))
	}
	count, err := store.BufferedObservationCount(ctx)
	if err != nil || count != 2 {
		t.Fatalf("buffered count = %d, %v", count, err)
	}
	if err := store.ConfirmObservations(ctx, []string{"first", "second"}); err != nil {
		t.Fatal(err)
	}
	buffered, err = store.BufferedObservations(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(buffered) != 0 {
		t.Fatalf("confirmed observations remain: %d", len(buffered))
	}
	count, err = store.BufferedObservationCount(ctx)
	if err != nil || count != 0 {
		t.Fatalf("buffered count after confirmation = %d, %v", count, err)
	}
}

func TestBatchSequenceAndBufferIsAtomic(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	first := ObservationDraft{DeviceID: "device", Build: func(sequence uint64) (BufferedObservation, error) {
		return BufferedObservation{ObservationID: "device-1", Payload: []byte{byte(sequence)}}, nil
	}}
	invalid := ObservationDraft{DeviceID: "device", Build: func(uint64) (BufferedObservation, error) {
		return BufferedObservation{}, nil
	}}
	if err := store.BufferObservationBatch(ctx, []ObservationDraft{first, invalid}); err == nil {
		t.Fatal("invalid batch accepted")
	}
	buffered, err := store.BufferedObservations(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(buffered) != 0 {
		t.Fatalf("failed batch persisted rows: %d", len(buffered))
	}
	if err := store.BufferObservationBatch(ctx, []ObservationDraft{first}); err != nil {
		t.Fatal(err)
	}
	buffered, err = store.BufferedObservations(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(buffered) != 1 || buffered[0].ObservationID != "device-1" || len(buffered[0].Payload) != 1 || buffered[0].Payload[0] != 1 {
		t.Fatalf("committed batch = %#v", buffered)
	}
}
