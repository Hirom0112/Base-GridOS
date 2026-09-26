package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestEventTransitionsAppendAudit(t *testing.T) {
	pool := testDatabase(t)
	insertEvent(t, pool, "event-transitions", "REQUESTED")
	states := []string{
		"REQUESTED",
		"PLANNED",
		"VALIDATED",
		"APPROVED",
		"COMMANDS_PERSISTED",
		"SENT",
		"ACKNOWLEDGED_OR_UNCERTAIN",
		"EXECUTING",
		"VERIFIED",
		"RECONCILED",
		"REPORTED",
	}
	for index := 0; index < len(states)-1; index++ {
		transitioned, err := TransitionEvent(context.Background(), pool, EventTransition{
			EventID:       "event-transitions",
			ExpectedState: states[index],
			NextState:     states[index+1],
			ActorID:       "operator-1",
			CorrelationID: "correlation-1",
			OccurredAt:    time.Now().UTC(),
		})
		if err != nil {
			t.Fatal(err)
		}
		if !transitioned {
			t.Fatalf("transition %s to %s did not occur", states[index], states[index+1])
		}
	}
	var auditCount int
	err := pool.QueryRow(context.Background(), "SELECT count(*) FROM audit_journal WHERE resource_id = $1", "event-transitions").Scan(&auditCount)
	if err != nil {
		t.Fatal(err)
	}
	if auditCount != len(states)-1 {
		t.Fatalf("audit rows = %d, want %d", auditCount, len(states)-1)
	}
}

func TestEventIllegalTransition(t *testing.T) {
	pool := testDatabase(t)
	insertEvent(t, pool, "event-illegal", "REQUESTED")
	transitioned, err := TransitionEvent(context.Background(), pool, EventTransition{
		EventID:       "event-illegal",
		ExpectedState: "REQUESTED",
		NextState:     "APPROVED",
		ActorID:       "operator-1",
		CorrelationID: "correlation-2",
		OccurredAt:    time.Now().UTC(),
	})
	if transitioned {
		t.Fatal("illegal transition occurred")
	}
	if !errors.Is(err, ErrIllegalTransition) {
		t.Fatalf("error = %v, want ErrIllegalTransition", err)
	}
}

func TestEventStaleExpectedStateAffectsNoRows(t *testing.T) {
	pool := testDatabase(t)
	insertEvent(t, pool, "event-stale", "PLANNED")
	transitioned, err := TransitionEvent(context.Background(), pool, EventTransition{
		EventID:       "event-stale",
		ExpectedState: "REQUESTED",
		NextState:     "PLANNED",
		ActorID:       "operator-1",
		CorrelationID: "correlation-3",
		OccurredAt:    time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if transitioned {
		t.Fatal("stale transition affected a row")
	}
	var auditCount int
	if err = pool.QueryRow(context.Background(), "SELECT count(*) FROM audit_journal WHERE resource_id = $1", "event-stale").Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 0 {
		t.Fatalf("audit rows = %d, want 0", auditCount)
	}
}

func insertEvent(t *testing.T, pool *pgxpool.Pool, eventID, state string) {
	t.Helper()
	ctx := context.Background()
	requestID := "request-" + eventID
	_, err := pool.Exec(ctx, `INSERT INTO dispatch_requests
        (request_id, event_type, begin_time, end_time, target_kw, measurement_boundary, load_zones, correlation_id)
        VALUES ($1, 'GRID_SERVICE', now(), now() + interval '1 hour', 100, 'METER_NET_EXPORT', ARRAY['LZ_HOUSTON'], 'test')`, requestID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO dispatch_events
        (event_id, request_id, state, correlation_id) VALUES ($1, $2, $3, 'test')`, eventID, requestID, state)
	if err != nil {
		t.Fatal(err)
	}
}
