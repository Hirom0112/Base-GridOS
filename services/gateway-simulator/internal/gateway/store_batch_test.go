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
}
