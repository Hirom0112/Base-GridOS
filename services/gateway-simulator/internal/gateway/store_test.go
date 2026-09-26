package gateway

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestCommandPersistenceAndIdempotency(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "gateway.db")
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 12, 18, 0, 0, 0, time.UTC)
	command := Command{
		CommandID:      "command-1",
		IdempotencyKey: "event-1-device-1-v1",
		DeviceID:       "device-1",
		Generation:     2,
		SetpointKW:     3.2,
		EffectiveAt:    now.Add(time.Minute),
		ExpiresAt:      now.Add(time.Hour),
	}
	ack, err := store.AcceptCommand(ctx, command, now)
	if err != nil {
		t.Fatal(err)
	}
	if !ack.Accepted || !ack.NewPhysicalEffect {
		t.Fatalf("ack=%+v", ack)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := reopened.Close(); err != nil {
			t.Error(err)
		}
	})
	commands, err := reopened.Commands(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(commands) != 1 || commands[0] != command {
		t.Fatalf("commands=%+v", commands)
	}
	assertCommandIdempotency(t, ctx, reopened, command, now)
	assertCommandEffectiveWindow(t, ctx, reopened, command, now)
}

func assertCommandIdempotency(t *testing.T, ctx context.Context, store *Store, command Command, now time.Time) {
	t.Helper()
	duplicate, err := store.AcceptCommand(ctx, command, now)
	if err != nil {
		t.Fatal(err)
	}
	if !duplicate.Accepted || duplicate.NewPhysicalEffect {
		t.Fatalf("duplicate=%+v", duplicate)
	}
	commands, err := store.Commands(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(commands) != 1 {
		t.Fatalf("command count=%d", len(commands))
	}
}

func assertCommandEffectiveWindow(t *testing.T, ctx context.Context, store *Store, command Command, now time.Time) {
	t.Helper()
	before, err := store.ExecutableCommands(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 0 {
		t.Fatalf("command executed before effective time: %+v", before)
	}
	after, err := store.ExecutableCommands(ctx, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 1 || after[0].CommandID != command.CommandID {
		t.Fatalf("effective commands=%+v", after)
	}
}

func TestCommandRejections(t *testing.T) {
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
	now := time.Date(2026, 8, 12, 18, 0, 0, 0, time.UTC)
	accepted := Command{
		CommandID:      "command-2",
		IdempotencyKey: "generation-2",
		DeviceID:       "device-1",
		Generation:     2,
		EffectiveAt:    now,
		ExpiresAt:      now.Add(time.Hour),
	}
	if ack, err := store.AcceptCommand(ctx, accepted, now); err != nil || !ack.Accepted {
		t.Fatalf("positive control ack=%+v err=%v", ack, err)
	}
	obsolete := accepted
	obsolete.CommandID = "command-1"
	obsolete.IdempotencyKey = "generation-1"
	obsolete.Generation = 1
	ack, err := store.AcceptCommand(ctx, obsolete, now)
	if err != nil {
		t.Fatal(err)
	}
	if ack.Accepted || ack.RejectionReason != "OBSOLETE_GENERATION" {
		t.Fatalf("obsolete ack=%+v", ack)
	}
	expired := accepted
	expired.CommandID = "command-3"
	expired.IdempotencyKey = "expired"
	expired.Generation = 3
	expired.EffectiveAt = now.Add(-time.Hour)
	expired.ExpiresAt = now.Add(-time.Second)
	ack, err = store.AcceptCommand(ctx, expired, now)
	if err != nil {
		t.Fatal(err)
	}
	if ack.Accepted || ack.RejectionReason != "EXPIRED" {
		t.Fatalf("expired ack=%+v", ack)
	}
}
