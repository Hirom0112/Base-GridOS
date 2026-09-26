package storage

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"pgregory.net/rapid"
)

func TestModelCommandStateMachine(t *testing.T) {
	pool := testDatabase(t)
	insertPlan(t, pool, "event-model")
	var sequence atomic.Int64
	operations := []string{"SEND", "ACK", "UNCERTAIN", "EXECUTE", "REJECT", "CANCEL", "EXPIRE", "COMPLETE"}
	rapid.Check(t, func(check *rapid.T) {
		id := fmt.Sprintf("model-%d", sequence.Add(1))
		command := testCommand(id, "event-model")
		if err := InsertCommand(context.Background(), pool, command); err != nil {
			check.Fatalf("insert command: %v", err)
		}
		state := "PERSISTED"
		steps := rapid.SliceOfN(rapid.SampledFrom(operations), 1, 30).Draw(check, "operations")
		for index, operation := range steps {
			next := modelNextState(state, operation)
			changed, err := applyModelOperation(context.Background(), pool, command, state, operation, index)
			if next == state {
				if err == nil && changed {
					check.Fatalf("illegal %s changed state %s", operation, state)
				}
			} else {
				if err != nil || !changed {
					check.Fatalf("legal %s from %s: changed=%v error=%v", operation, state, changed, err)
				}
				state = next
			}
			var databaseState string
			err = pool.QueryRow(context.Background(), `SELECT state FROM command_states
                WHERE command_id = $1 ORDER BY recorded_at DESC LIMIT 1`, command.CommandID).Scan(&databaseState)
			if err != nil {
				check.Fatalf("read state: %v", err)
			}
			if databaseState != state {
				check.Fatalf("database state = %s, model state = %s after %s", databaseState, state, operation)
			}
		}
	})
}

func applyModelOperation(ctx context.Context, pool *pgxpool.Pool, command CommandIntent, state, operation string, index int) (bool, error) {
	now := time.Now().UTC()
	switch operation {
	case "ACK":
		err := RecordAcknowledgement(ctx, pool, Acknowledgement{
			AcknowledgementID: fmt.Sprintf("ack-%s-%d", command.CommandID, index),
			CommandID:         command.CommandID,
			IdempotencyKey:    fmt.Sprintf("ack-idempotency-%s-%d", command.CommandID, index),
			ReceiptStatus:     "ACCEPTED",
			ReceivedAt:        now,
			GatewayID:         "gateway-model",
			CorrelationID:     command.CorrelationID,
		})
		return err == nil, err
	case "UNCERTAIN":
		return MarkAcknowledgementUncertain(ctx, pool, now, now.Add(time.Second), testFeasiblePowerInterval(command, now))
	default:
		return TransitionCommand(ctx, pool, CommandTransition{
			CommandID:     command.CommandID,
			ExpectedState: state,
			NextState:     modelOperationState(operation),
			OccurredAt:    now,
			CorrelationID: command.CorrelationID,
		})
	}
}

func modelNextState(state, operation string) string {
	transitions := map[string]map[string]string{
		"PERSISTED":    {"SEND": "SENT", "EXPIRE": "EXPIRED", "REJECT": "REJECTED"},
		"SENT":         {"ACK": "ACKNOWLEDGED", "UNCERTAIN": "UNCERTAIN", "EXPIRE": "EXPIRED", "REJECT": "REJECTED"},
		"ACKNOWLEDGED": {"EXECUTE": "EXECUTING", "CANCEL": "CANCELLED", "EXPIRE": "EXPIRED", "REJECT": "REJECTED"},
		"UNCERTAIN":    {"EXECUTE": "EXECUTING", "CANCEL": "CANCELLED", "EXPIRE": "EXPIRED", "REJECT": "REJECTED"},
		"EXECUTING":    {"COMPLETE": "COMPLETED", "CANCEL": "CANCELLED", "EXPIRE": "EXPIRED", "REJECT": "REJECTED"},
	}
	if next, ok := transitions[state][operation]; ok {
		return next
	}
	return state
}

func modelOperationState(operation string) string {
	states := map[string]string{
		"SEND": "SENT", "EXECUTE": "EXECUTING", "REJECT": "REJECTED",
		"CANCEL": "CANCELLED", "EXPIRE": "EXPIRED", "COMPLETE": "COMPLETED",
	}
	return states[operation]
}
