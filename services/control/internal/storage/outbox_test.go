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
}

func TestOutboxClaimersNeverOverlap(t *testing.T) {
	pool := testDatabase(t)
	insertPlan(t, pool, "event-claim")
	for index := 0; index < 6; index++ {
		if err := InsertCommand(context.Background(), pool, testCommand(fmt.Sprintf("claim-%d", index), "event-claim")); err != nil {
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
