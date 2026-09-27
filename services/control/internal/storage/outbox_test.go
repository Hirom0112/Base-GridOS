package storage

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestOutboxInsertIsAtomic(t *testing.T) {
	pool := testDatabase(t)
	insertPlan(t, pool, "event-atomic")
	_, err := pool.Exec(context.Background(), `
        CREATE FUNCTION fail_outbox_insert() RETURNS trigger LANGUAGE plpgsql AS $body$
        BEGIN RAISE EXCEPTION 'forced outbox failure'; END; $body$;
        CREATE TRIGGER force_outbox_failure BEFORE INSERT ON command_outbox
        FOR EACH ROW EXECUTE FUNCTION fail_outbox_insert()`)
	if err != nil {
		t.Fatal(err)
	}
	err = InsertCommand(context.Background(), pool, testCommand("atomic", "event-atomic"))
	if err == nil {
		t.Fatal("forced outbox failure succeeded")
	}
	var intents, outbox int
	if err = pool.QueryRow(context.Background(), "SELECT count(*) FROM command_intents").Scan(&intents); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(context.Background(), "SELECT count(*) FROM command_outbox").Scan(&outbox); err != nil {
		t.Fatal(err)
	}
	if intents != 0 || outbox != 0 {
		t.Fatalf("rows after rollback: intents=%d outbox=%d", intents, outbox)
	}
	var generations int
	if err = pool.QueryRow(context.Background(), "SELECT count(*) FROM device_command_generations").Scan(&generations); err != nil {
		t.Fatal(err)
	}
	if generations != 0 {
		t.Fatalf("generation counter survived rollback: %d", generations)
	}
}

func TestDeviceGenerationAcrossEvents(t *testing.T) {
	pool := testDatabase(t)
	insertPlan(t, pool, "generation-first")
	insertPlan(t, pool, "generation-second")
	ctx := context.Background()
	first := testCommand("first", "generation-first")
	if err := InsertCommand(ctx, pool, first); err != nil {
		t.Fatal(err)
	}
	second := testCommand("second", "generation-second")
	if err := InsertCommand(ctx, pool, second); err != nil {
		t.Fatal(err)
	}
	var firstGeneration, secondGeneration int64
	if err := pool.QueryRow(ctx, "SELECT generation FROM command_intents WHERE command_id=$1", first.CommandID).Scan(&firstGeneration); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT generation FROM command_intents WHERE command_id=$1", second.CommandID).Scan(&secondGeneration); err != nil {
		t.Fatal(err)
	}
	if firstGeneration != 1 || secondGeneration != 2 {
		t.Fatalf("consecutive device generations=%d,%d; want 1,2", firstGeneration, secondGeneration)
	}
	if err := InsertCommand(ctx, pool, second); err == nil {
		t.Fatal("duplicate command inserted")
	}
	var afterRetry int64
	if err := pool.QueryRow(ctx, "SELECT last_generation FROM device_command_generations WHERE device_id=$1", first.DeviceID).Scan(&afterRetry); err != nil {
		t.Fatal(err)
	}
	if afterRetry != 2 {
		t.Fatalf("duplicate insert consumed generation: %d", afterRetry)
	}
	stop := testCommand("stop", "generation-second")
	stop.Generation = 2
	stop.SetpointKW = 0
	if err := InsertZeroCommand(ctx, pool, stop); err != nil {
		t.Fatal(err)
	}
	var stopGeneration int64
	if err := pool.QueryRow(ctx, "SELECT generation FROM command_intents WHERE command_id=$1", stop.CommandID).Scan(&stopGeneration); err != nil {
		t.Fatal(err)
	}
	if stopGeneration != 3 {
		t.Fatalf("stop generation=%d; want 3", stopGeneration)
	}
	if err := InsertZeroCommand(ctx, pool, stop); err != nil {
		t.Fatal(err)
	}
	var lastGeneration int64
	if err := pool.QueryRow(ctx, "SELECT last_generation FROM device_command_generations WHERE device_id=$1", first.DeviceID).Scan(&lastGeneration); err != nil {
		t.Fatal(err)
	}
	if lastGeneration != 3 {
		t.Fatalf("retry consumed generation: %d", lastGeneration)
	}
}

func TestDeviceGenerationPublishesInOrder(t *testing.T) {
	pool := testDatabase(t)
	insertPlan(t, pool, "generation-order")
	ctx := context.Background()
	first := testCommand("z-first", "generation-order")
	second := testCommand("a-second", "generation-order")
	second.IssuedAt = first.IssuedAt
	if err := InsertCommand(ctx, pool, first); err != nil {
		t.Fatal(err)
	}
	if err := InsertCommand(ctx, pool, second); err != nil {
		t.Fatal(err)
	}
	claim := OutboxClaim{AvailableAt: first.IssuedAt.Add(time.Second), LeaseUntil: first.IssuedAt.Add(time.Minute), BatchSize: 1}
	claimed, err := ClaimOutbox(ctx, pool, claim)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 1 || claimed[0].CommandID != first.CommandID {
		t.Fatalf("first generation claim=%+v", claimed)
	}
	concurrent, err := ClaimOutbox(ctx, pool, claim)
	if err != nil {
		t.Fatal(err)
	}
	if len(concurrent) != 0 {
		t.Fatalf("concurrent claimant skipped predecessor: %+v", concurrent)
	}
	if err := MarkOutboxPublished(ctx, pool, first.CommandID, first.IssuedAt.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	next, err := ClaimOutbox(ctx, pool, claim)
	if err != nil {
		t.Fatal(err)
	}
	if len(next) != 1 || next[0].CommandID != second.CommandID {
		t.Fatalf("next generation claim=%+v", next)
	}
}

func TestDeviceGenerationStopBypassesPending(t *testing.T) {
	pool := testDatabase(t)
	insertPlan(t, pool, "generation-stop")
	ctx := context.Background()
	first := testCommand("generation-active", "generation-stop")
	stop := testCommand("generation-zero", "generation-stop")
	stop.SetpointKW = 0
	stop.IssuedAt = first.IssuedAt
	if err := InsertCommand(ctx, pool, first); err != nil {
		t.Fatal(err)
	}
	if err := InsertZeroCommand(ctx, pool, stop); err != nil {
		t.Fatal(err)
	}
	claim := OutboxClaim{AvailableAt: first.IssuedAt.Add(time.Second), LeaseUntil: first.IssuedAt.Add(time.Minute), BatchSize: 1}
	if _, err := ClaimOutbox(ctx, pool, claim); err != nil {
		t.Fatal(err)
	}
	claimed, err := ClaimOutbox(ctx, pool, claim)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 1 || claimed[0].CommandID != stop.CommandID {
		t.Fatalf("stop blocked by older generation: %+v", claimed)
	}
}

func TestDeviceGenerationTerminalPredecessorDoesNotBlock(t *testing.T) {
	pool := testDatabase(t)
	insertPlan(t, pool, "generation-terminal")
	ctx := context.Background()
	first := testCommand("a-expired", "generation-terminal")
	second := testCommand("z-next", "generation-terminal")
	second.IssuedAt = first.IssuedAt
	if err := InsertCommand(ctx, pool, first); err != nil {
		t.Fatal(err)
	}
	if err := InsertCommand(ctx, pool, second); err != nil {
		t.Fatal(err)
	}
	changed, err := TransitionCommand(ctx, pool, CommandTransition{CommandID: first.CommandID, ExpectedState: "PERSISTED", NextState: "EXPIRED", OccurredAt: first.IssuedAt.Add(time.Second), CorrelationID: first.CorrelationID})
	if err != nil || !changed {
		t.Fatalf("expire predecessor: changed=%t err=%v", changed, err)
	}
	claimed, err := ClaimOutbox(ctx, pool, OutboxClaim{AvailableAt: first.IssuedAt.Add(time.Second), LeaseUntil: first.IssuedAt.Add(time.Minute), BatchSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 1 || claimed[0].CommandID != second.CommandID {
		t.Fatalf("terminal predecessor blocked next generation: %+v", claimed)
	}
}

func TestDeviceGenerationUncertainPredecessorDoesNotBlock(t *testing.T) {
	pool := testDatabase(t)
	insertPlan(t, pool, "generation-uncertain")
	ctx := context.Background()
	first := testCommand("a-uncertain", "generation-uncertain")
	second := testCommand("z-next", "generation-uncertain")
	second.IssuedAt = first.IssuedAt
	if err := InsertCommand(ctx, pool, first); err != nil {
		t.Fatal(err)
	}
	if err := InsertCommand(ctx, pool, second); err != nil {
		t.Fatal(err)
	}
	claim := OutboxClaim{AvailableAt: first.IssuedAt.Add(time.Second), LeaseUntil: first.IssuedAt.Add(time.Minute), BatchSize: 1}
	claimed, err := ClaimOutbox(ctx, pool, claim)
	if err != nil || len(claimed) != 1 || claimed[0].CommandID != first.CommandID {
		t.Fatalf("first claim=%+v err=%v", claimed, err)
	}
	for _, transition := range []CommandTransition{
		{CommandID: first.CommandID, ExpectedState: "PERSISTED", NextState: "SENT", OccurredAt: first.IssuedAt.Add(time.Millisecond), CorrelationID: first.CorrelationID},
		{CommandID: first.CommandID, ExpectedState: "SENT", NextState: "UNCERTAIN", OccurredAt: first.IssuedAt.Add(2 * time.Millisecond), CorrelationID: first.CorrelationID},
	} {
		changed, err := TransitionCommand(ctx, pool, transition)
		if err != nil || !changed {
			t.Fatalf("uncertain transition: changed=%t err=%v", changed, err)
		}
	}
	claimed, err = ClaimOutbox(ctx, pool, claim)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 1 || claimed[0].CommandID != second.CommandID {
		t.Fatalf("uncertain predecessor blocked next generation: %+v", claimed)
	}
}

func TestOutboxClaimersNeverOverlap(t *testing.T) {
	pool := testDatabase(t)
	insertPlan(t, pool, "event-claim")
	for index := 0; index < 6; index++ {
		command := testCommand(fmt.Sprintf("claim-%d", index), "event-claim")
		command.DeviceID = fmt.Sprintf("device-%d", index)
		if err := InsertCommand(context.Background(), pool, command); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().UTC()
	results := make(chan []ClaimedCommand, 2)
	errors := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			claimed, err := ClaimOutbox(context.Background(), pool, OutboxClaim{
				AvailableAt: now,
				LeaseUntil:  now.Add(time.Minute),
				BatchSize:   6,
			})
			results <- claimed
			errors <- err
		}()
	}
	wait.Wait()
	close(results)
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	seen := make(map[string]bool)
	for batch := range results {
		for _, command := range batch {
			if seen[command.CommandID] {
				t.Fatalf("command %s claimed twice", command.CommandID)
			}
			seen[command.CommandID] = true
		}
	}
	if len(seen) != 6 {
		t.Fatalf("claimed %d commands, want 6", len(seen))
	}
}

func TestOutboxCrashReclaimPreservesCommand(t *testing.T) {
	pool := testDatabase(t)
	insertPlan(t, pool, "event-reclaim")
	command := testCommand("reclaim", "event-reclaim")
	if err := InsertCommand(context.Background(), pool, command); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	first, err := ClaimOutbox(context.Background(), pool, OutboxClaim{
		AvailableAt: now,
		LeaseUntil:  now.Add(time.Second),
		BatchSize:   1,
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := ClaimOutbox(context.Background(), pool, OutboxClaim{
		AvailableAt: now.Add(2 * time.Second),
		LeaseUntil:  now.Add(3 * time.Second),
		BatchSize:   1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("claim lengths = %d and %d, want 1 and 1", len(first), len(second))
	}
	if first[0].CommandIntent != second[0].CommandIntent {
		t.Fatalf("reclaimed command changed: %#v != %#v", first[0].CommandIntent, second[0].CommandIntent)
	}
	if second[0].CommandID != command.CommandID {
		t.Fatalf("command ID = %s, want %s", second[0].CommandID, command.CommandID)
	}
}

func insertPlan(t *testing.T, pool *pgxpool.Pool, eventID string) {
	t.Helper()
	insertEvent(t, pool, eventID, "APPROVED")
	ctx := context.Background()
	_, err := pool.Exec(ctx, `INSERT INTO input_snapshots
        (snapshot_id, event_id, captured_at, inputs, provenance, correlation_id)
        VALUES ($1, $2, now(), '{}', '{}', 'test')`, "input-"+eventID, eventID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO eligibility_snapshots
        (snapshot_id, event_id, captured_at, eligible_device_ids, exclusions, policy_version, correlation_id)
        VALUES ($1, $2, now(), ARRAY['device-1'], '{}', 'policy-1', 'test')`, "eligibility-"+eventID, eventID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO plan_versions
        (event_id, version, input_snapshot_id, eligibility_snapshot_id, plan, solver_version, model_version, correlation_id)
        VALUES ($1, 1, $2, $3, '{}', 'solver-1', 'model-1', 'test')`, eventID, "input-"+eventID, "eligibility-"+eventID)
	if err != nil {
		t.Fatal(err)
	}
}

func testCommand(id, eventID string) CommandIntent {
	now := time.Now().UTC()
	return CommandIntent{
		CommandID:      id,
		IdempotencyKey: "idempotency-" + id,
		DeviceID:       "device-1",
		EventID:        eventID,
		PlanVersion:    1,
		Generation:     1,
		SetpointKW:     3.5,
		IssuedAt:       now,
		EffectiveAt:    now.Add(time.Minute),
		ExpiresAt:      now.Add(time.Hour),
		PolicyVersion:  "policy-1",
		CorrelationID:  "correlation-" + id,
	}
}
