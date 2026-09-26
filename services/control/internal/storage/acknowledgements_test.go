package storage

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAcknowledgementDoesNotChangeEventState(t *testing.T) {
	pool := testDatabase(t)
	insertPlan(t, pool, "event-ack")
	command := testCommand("ack", "event-ack")
	if err := InsertCommand(context.Background(), pool, command); err != nil {
		t.Fatal(err)
	}
	appendCommandState(t, pool, command.CommandID, "SENT", command.CorrelationID)
	err := RecordAcknowledgement(context.Background(), pool, Acknowledgement{
		AcknowledgementID: "acknowledgement-1",
		CommandID:         command.CommandID,
		IdempotencyKey:    "ack-idempotency-1",
		ReceiptStatus:     "ACCEPTED",
		ReceivedAt:        time.Now().UTC(),
		GatewayID:         "gateway-1",
		CorrelationID:     command.CorrelationID,
	})
	if err != nil {
		t.Fatal(err)
	}
	var eventState string
	if err = pool.QueryRow(context.Background(), "SELECT state FROM dispatch_events WHERE event_id = $1", command.EventID).Scan(&eventState); err != nil {
		t.Fatal(err)
	}
	if eventState != "APPROVED" {
		t.Fatalf("event state = %s, want APPROVED", eventState)
	}
	if state := latestCommandState(t, pool, command.CommandID); state != "ACKNOWLEDGED" {
		t.Fatalf("command state = %s, want ACKNOWLEDGED", state)
	}
}

func TestAckDeadlineMarksUncertainWithInterval(t *testing.T) {
	pool := testDatabase(t)
	insertPlan(t, pool, "event-uncertain")
	command := testCommand("uncertain", "event-uncertain")
	if err := InsertCommand(context.Background(), pool, command); err != nil {
		t.Fatal(err)
	}
	appendCommandState(t, pool, command.CommandID, "SENT", command.CorrelationID)
	deadline := time.Now().UTC()
	interval := testFeasiblePowerInterval(command, deadline)
	changed, err := MarkAcknowledgementUncertain(context.Background(), pool, deadline, deadline.Add(time.Second), interval)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("unacknowledged command was not marked uncertain")
	}
	if state := latestCommandState(t, pool, command.CommandID); state != "UNCERTAIN" {
		t.Fatalf("command state = %s, want UNCERTAIN", state)
	}
	var lower, upper float64
	err = pool.QueryRow(context.Background(), `SELECT signed_feasible_power_lower_kw,
        signed_feasible_power_upper_kw FROM uncertainty_intervals
        WHERE possibly_accepted_command_id = $1`, command.CommandID).Scan(&lower, &upper)
	if err != nil {
		t.Fatal(err)
	}
	if lower != -2.5 || upper != 3.5 {
		t.Fatalf("stored interval = [%v, %v], want [-2.5, 3.5]", lower, upper)
	}
}

func TestLateAcceptedAcknowledgementResolvesUncertainCommand(t *testing.T) {
	pool := testDatabase(t)
	insertPlan(t, pool, "event-late-ack")
	command := testCommand("late-ack", "event-late-ack")
	if err := InsertCommand(context.Background(), pool, command); err != nil {
		t.Fatal(err)
	}
	appendCommandState(t, pool, command.CommandID, "SENT", command.CorrelationID)
	deadline := time.Now().UTC()
	if _, err := MarkAcknowledgementUncertain(context.Background(), pool, deadline, deadline, testFeasiblePowerInterval(command, deadline)); err != nil {
		t.Fatal(err)
	}
	if err := RecordAcknowledgement(context.Background(), pool, Acknowledgement{
		AcknowledgementID: "late-acknowledgement", CommandID: command.CommandID,
		IdempotencyKey: command.IdempotencyKey, ReceiptStatus: "ACCEPTED",
		ReceivedAt: deadline.Add(time.Second), GatewayID: "gateway-1", CorrelationID: command.CorrelationID,
	}); err != nil {
		t.Fatal(err)
	}
	if state := latestCommandState(t, pool, command.CommandID); state != "ACKNOWLEDGED" {
		t.Fatalf("command state = %s, want ACKNOWLEDGED", state)
	}
}

func TestAcknowledgedCommandIsNotMarkedUncertain(t *testing.T) {
	pool := testDatabase(t)
	insertPlan(t, pool, "event-ack-before-deadline")
	command := testCommand("ack-before-deadline", "event-ack-before-deadline")
	if err := InsertCommand(context.Background(), pool, command); err != nil {
		t.Fatal(err)
	}
	appendCommandState(t, pool, command.CommandID, "SENT", command.CorrelationID)
	deadline := time.Now().UTC()
	if err := RecordAcknowledgement(context.Background(), pool, Acknowledgement{
		AcknowledgementID: "acknowledgement-2",
		CommandID:         command.CommandID,
		IdempotencyKey:    "ack-idempotency-2",
		ReceiptStatus:     "ACCEPTED",
		ReceivedAt:        deadline.Add(-time.Second),
		GatewayID:         "gateway-1",
		CorrelationID:     command.CorrelationID,
	}); err != nil {
		t.Fatal(err)
	}
	changed, err := MarkAcknowledgementUncertain(context.Background(), pool, deadline, deadline.Add(time.Second), testFeasiblePowerInterval(command, deadline))
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("acknowledged command was marked uncertain")
	}
	if state := latestCommandState(t, pool, command.CommandID); state != "ACKNOWLEDGED" {
		t.Fatalf("command state = %s, want ACKNOWLEDGED", state)
	}
}

func appendCommandState(t *testing.T, pool *pgxpool.Pool, commandID, state, correlationID string) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `INSERT INTO command_states
        (command_id, state, recorded_at, correlation_id) VALUES ($1, $2, now(), $3)`, commandID, state, correlationID)
	if err != nil {
		t.Fatal(err)
	}
}

func latestCommandState(t *testing.T, pool *pgxpool.Pool, commandID string) string {
	t.Helper()
	var state string
	err := pool.QueryRow(context.Background(), `SELECT state FROM command_states
        WHERE command_id = $1 ORDER BY recorded_at DESC LIMIT 1`, commandID).Scan(&state)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func testFeasiblePowerInterval(command CommandIntent, now time.Time) FeasiblePowerInterval {
	return FeasiblePowerInterval{
		DeviceID:                    command.DeviceID,
		IntervalBegin:               now,
		IntervalEnd:                 now.Add(5 * time.Minute),
		LowerKW:                     -2.5,
		UpperKW:                     3.5,
		LastConfirmedCommandID:      "prior-command",
		LastConfirmedSetpointKW:     0,
		PossiblyAcceptedCommandID:   command.CommandID,
		PossiblyAcceptedSetpointKW:  command.SetpointKW,
		PossiblyAcceptedEffectiveAt: command.EffectiveAt,
		PossiblyAcceptedExpiresAt:   command.ExpiresAt,
		MaxRampKWPerSecond:          1,
		FreshTelemetryPowerKW:       0.5,
		FreshTelemetryObservedAt:    now,
		DerivedAt:                   now,
		CorrelationID:               command.CorrelationID,
	}
}
