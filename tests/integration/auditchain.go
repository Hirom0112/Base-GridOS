package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"testing"
)

type transition struct {
	previous string
	next     string
}

type eventStateValue struct {
	State string `json:"state"`
}

var legalCommandTransitions = map[string][]string{
	"PERSISTED":    {"SENT", "EXPIRED", "REJECTED"},
	"SENT":         {"ACKNOWLEDGED", "UNCERTAIN", "EXPIRED", "REJECTED"},
	"ACKNOWLEDGED": {"EXECUTING", "CANCELLED", "EXPIRED", "REJECTED"},
	"UNCERTAIN":    {"ACKNOWLEDGED", "EXECUTING", "REJECTED", "CANCELLED", "EXPIRED"},
	"EXECUTING":    {"COMPLETED", "CANCELLED", "EXPIRED", "REJECTED"},
}

func chainBreak(transitions []transition, last string) error {
	if len(transitions) == 0 {
		return errors.New("no transitions recorded")
	}
	if transitions[0].previous != "REQUESTED" {
		return fmt.Errorf("chain starts at %s, not REQUESTED", transitions[0].previous)
	}
	for index := 1; index < len(transitions); index++ {
		if transitions[index].previous != transitions[index-1].next {
			return fmt.Errorf("transition %d leaves %s but the chain was at %s", index, transitions[index].previous, transitions[index-1].next)
		}
	}
	if final := transitions[len(transitions)-1].next; final != last {
		return fmt.Errorf("chain ends at %s, stored state is %s", final, last)
	}
	return nil
}

func commandChainBreak(states []string) error {
	if len(states) == 0 || states[0] != "PERSISTED" {
		return fmt.Errorf("command chain %v does not start at PERSISTED", states)
	}
	for index := 1; index < len(states); index++ {
		if !slices.Contains(legalCommandTransitions[states[index-1]], states[index]) {
			return fmt.Errorf("command chain %v moves %s to %s", states, states[index-1], states[index])
		}
	}
	return nil
}

func (stack *stack) assertAuditChain(t *testing.T, ctx context.Context, eventID string) int {
	t.Helper()
	var storedState, correlationID string
	if err := stack.pool.QueryRow(ctx, "SELECT state, correlation_id FROM dispatch_events WHERE event_id = $1", eventID).Scan(&storedState, &correlationID); err != nil {
		t.Fatal(err)
	}
	var firstAction string
	err := stack.pool.QueryRow(ctx, `SELECT action FROM audit_journal WHERE resource_type = 'dispatch_event' AND resource_id = $1
		ORDER BY sequence LIMIT 1`, eventID).Scan(&firstAction)
	if err != nil || firstAction != "EVENT_REQUEST_CREATED" {
		t.Fatalf("first audit action for %s = %q, %v", eventID, firstAction, err)
	}
	if err = chainBreak(stack.eventTransitions(t, ctx, eventID), storedState); err != nil {
		t.Fatalf("event %s audit chain: %v", eventID, err)
	}
	var foreign int
	if err = stack.pool.QueryRow(ctx, "SELECT count(*) FROM audit_journal WHERE resource_id = $1 AND correlation_id <> $2", eventID, correlationID).Scan(&foreign); err != nil {
		t.Fatal(err)
	}
	if foreign != 0 {
		t.Fatalf("%d audit rows for %s carry a foreign correlation", foreign, eventID)
	}
	return stack.assertCommandChains(t, ctx, eventID, correlationID)
}

func (stack *stack) eventTransitions(t *testing.T, ctx context.Context, eventID string) []transition {
	t.Helper()
	rows, err := stack.pool.Query(ctx, `SELECT previous_values, new_values FROM audit_journal
		WHERE resource_type = 'dispatch_event' AND resource_id = $1 AND action = 'EVENT_STATE_TRANSITIONED' ORDER BY sequence`, eventID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	transitions := make([]transition, 0)
	for rows.Next() {
		var previousValues, newValues []byte
		var previous, next eventStateValue
		if err = rows.Scan(&previousValues, &newValues); err != nil {
			t.Fatal(err)
		}
		if err = errors.Join(json.Unmarshal(previousValues, &previous), json.Unmarshal(newValues, &next)); err != nil {
			t.Fatal(err)
		}
		transitions = append(transitions, transition{previous: previous.State, next: next.State})
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	return transitions
}

func (stack *stack) assertCommandChains(t *testing.T, ctx context.Context, eventID, correlationID string) int {
	t.Helper()
	rows, err := stack.pool.Query(ctx, `SELECT intent.command_id, intent.correlation_id, array_agg(state.state ORDER BY state.recorded_at)
		FROM command_intents AS intent JOIN command_states AS state USING (command_id)
		WHERE intent.event_id = $1 GROUP BY intent.command_id, intent.correlation_id ORDER BY intent.command_id`, eventID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	checked := 0
	for rows.Next() {
		var commandID, commandCorrelation string
		var states []string
		if err = rows.Scan(&commandID, &commandCorrelation, &states); err != nil {
			t.Fatal(err)
		}
		if commandCorrelation != correlationID {
			t.Fatalf("command %s correlation %s is not the event's %s", commandID, commandCorrelation, correlationID)
		}
		if err = commandChainBreak(states); err != nil {
			t.Fatalf("command %s: %v", commandID, err)
		}
		checked++
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	return checked
}
